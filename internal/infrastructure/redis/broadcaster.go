package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	messagesvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
)

const channelPrefix = "chat:"

// RedisBroadcaster реализует интерфейс Broadcaster поверх Redis Pub/Sub.
type RedisBroadcaster struct {
	pool *redis.Pool
	log  *slog.Logger
}

func NewRedisBroadcaster(pool *redis.Pool, log *slog.Logger) *RedisBroadcaster {
	return &RedisBroadcaster{pool: pool, log: log}
}

func (b *RedisBroadcaster) Publish(
	ctx context.Context,
	chatID uuid.UUID,
	msg messagesvc.Message,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := json.Marshal(toWire(msg))
	if err != nil {
		return fmt.Errorf("marshal broadcast message: %w", err)
	}

	conn := b.pool.Get()
	defer func() { _ = conn.Close() }()

	if _, err = conn.Do("PUBLISH", channelName(chatID), payload); err != nil {
		return fmt.Errorf("publish broadcast message: %w", err)
	}

	return nil
}

func (b *RedisBroadcaster) Subscribe(
	ctx context.Context,
	chatID uuid.UUID,
) (<-chan messagesvc.Message, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	conn, err := b.pool.Dial()
	if err != nil {
		return nil, nil, fmt.Errorf("dial redis for subscription: %w", err)
	}

	psc := redis.PubSubConn{Conn: conn}
	channel := channelName(chatID)

	if err = psc.Subscribe(channel); err != nil {
		_ = conn.Close()

		return nil, nil, fmt.Errorf("subscribe to %s: %w", channel, err)
	}

	if err = waitSubscription(&psc, channel); err != nil {
		_ = conn.Close()

		return nil, nil, fmt.Errorf("wait subscription to %s: %w", channel, err)
	}

	out := make(chan messagesvc.Message)
	stop := make(chan struct{})

	var once sync.Once

	unsubscribe := func() {
		once.Do(func() {
			close(stop)
			_ = psc.Unsubscribe(channel)
			_ = conn.Close()
		})
	}

	go func() {
		select {
		case <-ctx.Done():
			unsubscribe()
		case <-stop:
		}
	}()

	go func() {
		defer close(out)
		defer func() { _ = conn.Close() }()

		for {
			event := psc.Receive()

			switch value := event.(type) {
			case redis.Message:
				var wire wireMessage
				if err := json.Unmarshal(value.Data, &wire); err != nil {
					b.log.Warn("invalid broadcast payload", "error", err, "chat_id", chatID)

					continue
				}

				select {
				case out <- wire.toDomain():
				case <-stop:
					return
				}
			case redis.Subscription:
				continue
			case error:
				select {
				case <-stop:
					return
				default:
					b.log.Warn("redis subscription error", "error", value, "chat_id", chatID)

					return
				}
			}
		}
	}()

	return out, unsubscribe, nil
}

func waitSubscription(psc *redis.PubSubConn, channel string) error {
	for {
		switch value := psc.Receive().(type) {
		case redis.Subscription:
			if value.Channel == channel && value.Kind == "subscribe" {
				return nil
			}
		case error:
			return value
		}
	}
}

func channelName(chatID uuid.UUID) string {
	return channelPrefix + chatID.String()
}

type wireMessage struct {
	ID        uuid.UUID `json:"id"`
	ChatID    uuid.UUID `json:"chat_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

func toWire(msg messagesvc.Message) wireMessage {
	return wireMessage{
		ID:        msg.ID,
		ChatID:    msg.ChatID,
		AuthorID:  msg.AuthorID,
		Text:      msg.Text,
		CreatedAt: msg.CreatedAt,
	}
}

func (w wireMessage) toDomain() messagesvc.Message {
	return messagesvc.Message{
		ID:        w.ID,
		ChatID:    w.ChatID,
		AuthorID:  w.AuthorID,
		Text:      w.Text,
		CreatedAt: w.CreatedAt,
	}
}

var _ messagesvc.Broadcaster = (*RedisBroadcaster)(nil)
