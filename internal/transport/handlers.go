package transport

import (
	"errors"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/aggregate"
	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	messagesvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	usersvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
)

const (
	msgNotInChat = "You are not in a chat. Disconnecting.\n"
	msgInternal  = "Internal error.\n"
)

func (s *session) handle(cmd command) bool {
	if cmd.chat {
		return s.handleChatCommand(cmd)
	}

	switch cmd.name {
	case cmdWhoami:
		s.out.writeString("You are " + s.user.Nickname + ".\n")
	case cmdChatsList:
		s.handleChatsList()
	case cmdChatsCreate:
		s.handleChatsCreate()
	case cmdChatsJoin:
		s.handleChatsJoin(cmd.arg)
	case cmdChatsDelete:
		s.handleChatsDelete()
	default:
		s.out.writeString("Unknown command. Try /whoami or /chats-list.\n")
	}

	return true
}

func (s *session) handleChatCommand(cmd command) bool {
	if s.active == nil {
		s.out.writeString(msgNotInChat)

		return false
	}

	member, err := s.chats.IsMember(s.sess.Context(), s.active.chat.ID, s.user.ID)
	if err != nil {
		s.writeError(err)

		return true
	}
	if !member {
		s.out.writeString(msgNotInChat)

		return false
	}

	switch cmd.name {
	case cmdSend:
		return s.handleSend(cmd.text)
	case cmdPrevMsg:
		s.handlePrevMsg()
	case cmdBan:
		s.handleBan(cmd.arg)
	default:
		s.out.writeString("Unknown @chat command.\n")
	}

	return true
}

func (s *session) handleSend(text string) bool {
	if text == "" {
		s.out.writeString("Usage: @chat /send <text>\n")

		return true
	}

	if _, err := s.agg.SendMessage(s.sess.Context(), s.active.chat.ID, s.user.ID, text); err != nil {
		if errors.Is(err, aggregate.ErrNotInChat) {
			s.out.writeString(msgNotInChat)

			return false
		}

		s.writeError(err)

		return true
	}

	// Сообщение придёт обратно через подписку; локальный echo не нужен.
	return true
}

func (s *session) handlePrevMsg() {
	messages, err := s.agg.History(s.sess.Context(), s.active.chat.ID, nil, messagesvc.DefaultHistoryLimit)
	if err != nil {
		s.writeError(err)

		return
	}
	if len(messages) == 0 {
		s.out.writeString("No messages yet.\n")

		return
	}

	for i := len(messages) - 1; i >= 0; i-- {
		s.out.writeString(formatMessage(messages[i]))
	}
}

func (s *session) handleBan(nickname string) {
	if nickname == "" {
		s.out.writeString("Usage: @chat /ban <nickname>\n")

		return
	}

	target, err := s.users.ByNickname(s.sess.Context(), nickname)
	if err != nil {
		s.writeError(err)

		return
	}

	if err = s.chats.Ban(s.sess.Context(), s.active.chat.ID, s.user.ID, target.ID); err != nil {
		s.writeError(err)

		return
	}

	s.out.writeString("Banned " + target.Nickname + ".\n")
}

func (s *session) handleChatsList() {
	chats, err := s.chats.ListByUser(s.sess.Context(), s.user.ID)
	if err != nil {
		s.writeError(err)

		return
	}
	if len(chats) == 0 {
		s.out.writeString("You are not in any chats.\n")

		return
	}

	for _, chat := range chats {
		s.out.writeString(formatChat(chat, s.user.ID))
	}
}

func (s *session) handleChatsCreate() {
	chat, err := s.chats.Create(s.sess.Context(), s.user.ID)
	if err != nil {
		s.writeError(err)

		return
	}

	s.enterChat(chat)
	s.out.writeString("Chat created.\nToken: " + chat.Token.String() + "\n")
}

func (s *session) handleChatsJoin(token string) {
	if token == "" {
		s.out.writeString("Usage: /chats-join <token>\n")

		return
	}

	chatID, err := uuid.Parse(token)
	if err != nil {
		s.out.writeString("Invalid chat token.\n")

		return
	}

	chat, err := s.chats.Join(s.sess.Context(), chatID, s.user.ID)
	if err != nil {
		s.writeError(err)

		return
	}

	s.enterChat(chat)
	s.out.writeString("Joined chat " + chat.ID.String() + ".\n")
	s.showHistory()
}

func (s *session) handleChatsDelete() {
	if s.active == nil {
		s.out.writeString("You are not in a chat.\n")

		return
	}

	if err := s.chats.Delete(s.sess.Context(), s.active.chat.ID, s.user.ID); err != nil {
		if errors.Is(err, chatsvc.ErrChatNotFound) {
			s.leaveChat()
		}

		s.writeError(err)

		return
	}

	s.leaveChat()
	s.out.writeString("Chat deleted.\n")
}

func (s *session) showHistory() {
	messages, err := s.agg.History(s.sess.Context(), s.active.chat.ID, nil, messagesvc.DefaultHistoryLimit)
	if err != nil {
		s.writeError(err)

		return
	}
	if len(messages) == 0 {
		return
	}

	for i := len(messages) - 1; i >= 0; i-- {
		s.out.writeString(formatMessage(messages[i]))
	}
}

func (s *session) writeError(err error) {
	switch {
	case errors.Is(err, messagesvc.ErrMessageTooLong):
		s.out.writeString("Message is too long.\n")
	case errors.Is(err, messagesvc.ErrEmptyMessage):
		s.out.writeString("Message is empty.\n")
	case errors.Is(err, chatsvc.ErrChatFull):
		s.out.writeString("Chat is full.\n")
	case errors.Is(err, chatsvc.ErrBanned):
		s.out.writeString("You are banned in this chat.\n")
	case errors.Is(err, chatsvc.ErrNotHost):
		s.out.writeString("Only the chat host can do that.\n")
	case errors.Is(err, chatsvc.ErrCannotBanHost):
		s.out.writeString("You cannot ban the chat host.\n")
	case errors.Is(err, chatsvc.ErrUserNotMember):
		s.out.writeString("User is not a member of this chat.\n")
	case errors.Is(err, chatsvc.ErrChatNotFound):
		s.out.writeString("Chat not found.\n")
	case errors.Is(err, chatsvc.ErrInvalidToken):
		s.out.writeString("Invalid chat token.\n")
	case errors.Is(err, usersvc.ErrUserNotFound):
		s.out.writeString("User not found.\n")
	case errors.Is(err, usersvc.ErrInvalidNickname):
		s.out.writeString("Invalid nickname.\n")
	case errors.Is(err, usersvc.ErrNickTaken):
		s.out.writeString("Nickname is already taken.\n")
	case errors.Is(err, aggregate.ErrNotInChat):
		s.out.writeString(msgNotInChat)
	default:
		s.log.Error("ssh command failed", "error", err, "user_id", s.user.ID, "nickname", s.user.Nickname)
		s.out.writeString(msgInternal)
	}
}
