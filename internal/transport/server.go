package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/config"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/aggregate"
	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	usersvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/skevetter/ssh"
)

// Server — SSH-транспорт приложения.
type Server struct {
	cfg   config.Config
	log   *slog.Logger
	srv   *ssh.Server
	ln    net.Listener
	users usersvc.Service
	chats chatsvc.Service
	agg   *aggregate.Aggregate
}

func NewServer(
	cfg config.Config,
	log *slog.Logger,
	users usersvc.Service,
	chats chatsvc.Service,
	agg *aggregate.Aggregate,
) (*Server, error) {
	s := &Server{
		cfg:   cfg,
		log:   log,
		users: users,
		chats: chats,
		agg:   agg,
	}

	s.srv = &ssh.Server{
		Handler:          s.handleSession,
		PublicKeyHandler: s.handlePublicKey,
	}

	if cfg.SSHHostKeyPath != "" {
		if err := s.srv.SetOption(ssh.HostKeyFile(cfg.SSHHostKeyPath)); err != nil {
			return nil, fmt.Errorf("load ssh host key: %w", err)
		}
	}

	ln, err := net.Listen("tcp", cfg.SSHAddr)
	if err != nil {
		return nil, fmt.Errorf("listen ssh on %s: %w", cfg.SSHAddr, err)
	}
	s.ln = ln

	return s, nil
}

// Serve блокируется до остановки сервера.
func (s *Server) Serve() error {
	s.log.Info("ssh server listening", "addr", s.ln.Addr().String())

	err := s.srv.Serve(s.ln)
	if errors.Is(err, ssh.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}

	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) Close() error {
	_ = s.ln.Close()

	return s.srv.Close()
}

func (s *Server) handlePublicKey(_ ssh.Context, key ssh.PublicKey) bool {
	s.log.Debug("ssh public key auth", "fingerprint", fingerprint(key))

	return key != nil
}

func (s *Server) handleSession(sess ssh.Session) {
	session := newSession(s, sess, fingerprint(sess.PublicKey()))
	session.run()

	_ = sess.Exit(0)
}

func fingerprint(key ssh.PublicKey) string {
	if key == nil {
		return ""
	}

	sum := sha256.Sum256(key.Marshal())

	return hex.EncodeToString(sum[:])
}
