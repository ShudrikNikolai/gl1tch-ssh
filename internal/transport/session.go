package transport

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/aggregate"
	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	usersvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
	"github.com/skevetter/ssh"
)

const (
	maxInputBytes = 4096
	prompt        = "> "
)

type sessionWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *sessionWriter) writeString(value string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, _ = io.WriteString(w.writer, value)
}

type activeChat struct {
	chat        chatsvc.Chat
	unsubscribe func()
	done        chan struct{}
}

type session struct {
	sess        ssh.Session
	log         *slog.Logger
	users       usersvc.Service
	chats       chatsvc.Service
	agg         *aggregate.Aggregate
	out         *sessionWriter
	reader      *bufio.Reader
	pty         bool
	skipLF      bool
	fingerprint string
	user        usersvc.User
	active      *activeChat
}

func newSession(server *Server, sess ssh.Session, fingerprint string) *session {
	_, _, pty := sess.Pty()

	return &session{
		sess:        sess,
		log:         server.log,
		users:       server.users,
		chats:       server.chats,
		agg:         server.agg,
		out:         &sessionWriter{writer: sess},
		reader:      bufio.NewReader(sess),
		pty:         pty,
		fingerprint: fingerprint,
	}
}

func (s *session) run() {
	defer s.leaveChat()

	signals := make(chan ssh.Signal, 1)
	s.sess.Signals(signals)
	defer s.sess.Signals(nil)

	go s.watchSignals(signals)

	s.out.writeString("gl1tch-ssh\n")

	user, ok := s.authenticate()
	if !ok {
		return
	}
	s.user = user

	s.out.writeString("Welcome, " + user.Nickname + "!\n")
	s.repl()
}

func (s *session) authenticate() (usersvc.User, bool) {
	user, err := s.users.ByFingerprint(s.sess.Context(), s.fingerprint)
	if err == nil {
		return user, true
	}
	if !errors.Is(err, usersvc.ErrUserNotFound) {
		s.log.Error("failed to authenticate ssh user", "error", err)
		s.out.writeString("Authentication failed. Disconnecting.\n")

		return usersvc.User{}, false
	}

	return s.register()
}

func (s *session) register() (usersvc.User, bool) {
	for {
		s.out.writeString("Choose a nickname (3-18 chars, A-Z a-z 0-9 _ -): ")

		line, ok := s.readLine()
		if !ok {
			return usersvc.User{}, false
		}

		user, err := s.users.Register(s.sess.Context(), s.fingerprint, strings.TrimSpace(line))
		if err == nil {
			return user, true
		}

		switch {
		case errors.Is(err, usersvc.ErrInvalidNickname):
			s.out.writeString("Invalid nickname. Try again.\n")
		case errors.Is(err, usersvc.ErrNickTaken):
			s.out.writeString("Nickname is already taken. Try another one.\n")
		default:
			s.log.Error("failed to register ssh user", "error", err)
			s.out.writeString("Registration failed. Disconnecting.\n")

			return usersvc.User{}, false
		}
	}
}

func (s *session) repl() {
	for {
		s.out.writeString(prompt)

		line, ok := s.readLine()
		if !ok {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !s.handle(parseCommand(line)) {
			return
		}
	}
}

func (s *session) readLine() (string, bool) {
	line, err := s.readInputLine()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			s.log.Debug("ssh session read stopped", "error", err)
		}

		return "", false
	}

	return line, true
}

// readInputLine читает строку, принимая и \n, и \r (интерактивный Enter в PTY),
// и эхо-выводит ввод, потому что клиент в raw-режиме сам его не показывает.
func (s *session) readInputLine() (string, error) {
	line := make([]rune, 0, 64)

	for {
		r, _, err := s.reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) && len(line) > 0 {
				return string(line), nil
			}

			return "", err
		}

		if s.skipLF {
			s.skipLF = false
			if r == '\n' {
				continue
			}
		}

		switch r {
		case '\r':
			// Не пытаемся читать следующий байт: в интерактивном PTY его ещё нет.
			// LF от CRLF пропустим в начале следующего чтения.
			s.skipLF = true
			s.echo("\n")

			return string(line), nil
		case '\n':
			s.echo("\n")

			return string(line), nil
		case 3: // Ctrl+C
			s.echo("^C\n")

			return "", io.EOF
		case 4: // Ctrl+D
			if len(line) == 0 {
				return "", io.EOF
			}
		case 8, 127: // Backspace
			if len(line) > 0 {
				line = line[:len(line)-1]
				s.echo("\b \b")
			}
		default:
			if r < 32 || len(line) >= maxInputBytes {
				continue
			}

			line = append(line, r)
			s.echo(string(r))
		}
	}
}

func (s *session) echo(value string) {
	if s.pty {
		s.out.writeString(value)
	}
}

func (s *session) watchSignals(signals <-chan ssh.Signal) {
	for {
		select {
		case sig := <-signals:
			if sig == ssh.SIGINT || sig == ssh.SIGTERM || sig == ssh.SIGQUIT {
				_ = s.sess.Close()

				return
			}
		case <-s.sess.Context().Done():
			return
		}
	}
}

func (s *session) enterChat(chat chatsvc.Chat) {
	s.leaveChat()

	s.active = &activeChat{chat: chat}

	messages, unsubscribe, err := s.agg.Subscribe(s.sess.Context(), chat.ID)
	if err != nil {
		s.log.Error("failed to subscribe to chat", "error", err, "chat_id", chat.ID)
		s.out.writeString("Live updates are unavailable.\n")

		return
	}

	done := make(chan struct{})
	s.active.unsubscribe = unsubscribe
	s.active.done = done

	go s.readBroadcasts(chat.ID, messages, done)
}

func (s *session) leaveChat() {
	if s.active == nil {
		return
	}

	if s.active.unsubscribe != nil {
		s.active.unsubscribe()
	}
	if s.active.done != nil {
		close(s.active.done)
	}

	s.active = nil
}

func (s *session) readBroadcasts(
	chatID uuid.UUID,
	messages <-chan aggregate.ChatMessage,
	done <-chan struct{},
) {
	for {
		select {
		case <-done:
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}

			member, err := s.chats.IsMember(s.sess.Context(), chatID, s.user.ID)
			if err != nil {
				s.log.Error("failed to check chat membership", "error", err, "chat_id", chatID)
			} else if !member {
				s.out.writeString(msgNotInChat)
				_ = s.sess.Close()

				return
			}

			select {
			case <-done:
				return
			default:
			}

			s.out.writeString(formatMessage(msg))
		}
	}
}
