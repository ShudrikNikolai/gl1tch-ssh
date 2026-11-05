//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/infrastructure/pg/migrations"
	chatsrepoErrors "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository"
	chatsrepo "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository/repo"
	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	messagesrepo "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/repository/repo"
	messagesvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	usersrepoErrors "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository"
	usersrepo "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository/repo"
	usersvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestRepositories(t *testing.T) {
	if testing.Short() {
		t.Skip("integration tests are skipped in short mode")
	}

	// Ryuk (resource reaper) часто недоступен в Docker Desktop на Windows;
	// контейнер всё равно удаляется через CleanupContainer.
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	ctx := context.Background()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:18.3",
		postgres.WithDatabase("gl1tch"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	testcontainers.CleanupContainer(t, pgContainer)

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres dsn: %v", err)
	}

	applyMigrations(t, dsn)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)

	t.Run("users", func(t *testing.T) { testUsers(t, pool) })
	t.Run("chats", func(t *testing.T) { testChats(t, pool) })
	t.Run("messages", func(t *testing.T) { testMessages(t, pool) })
}

func applyMigrations(t *testing.T, dsn string) {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open migration db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations.FS,
		goose.WithTableName("schema_migrations"),
	)
	if err != nil {
		t.Fatalf("create migration provider: %v", err)
	}

	if _, err = provider.Up(context.Background()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}

func testUsers(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	repo := usersrepo.New(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := usersvc.User{
		ID:          uuid.New(),
		Nickname:    "alice",
		Fingerprint: strings.Repeat("ab", 32),
		CreatedAt:   now,
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	byFingerprint, err := repo.FindByFingerprint(ctx, user.Fingerprint)
	if err != nil {
		t.Fatalf("find by fingerprint: %v", err)
	}
	if byFingerprint.ID != user.ID ||
		byFingerprint.Nickname != user.Nickname ||
		byFingerprint.Fingerprint != user.Fingerprint ||
		!byFingerprint.CreatedAt.Equal(user.CreatedAt) {
		t.Fatalf("by fingerprint = %+v, want %+v", byFingerprint, user)
	}

	byNickname, err := repo.FindByNickname(ctx, user.Nickname)
	if err != nil {
		t.Fatalf("find by nickname: %v", err)
	}
	if byNickname.ID != user.ID {
		t.Fatalf("by nickname = %+v", byNickname)
	}

	byIDs, err := repo.GetByIDs(ctx, []uuid.UUID{user.ID, uuid.New()})
	if err != nil {
		t.Fatalf("get by ids: %v", err)
	}
	if len(byIDs) != 1 || byIDs[user.ID].Nickname != user.Nickname {
		t.Fatalf("by ids = %+v", byIDs)
	}

	duplicateNickname := usersvc.User{
		ID:          uuid.New(),
		Nickname:    user.Nickname,
		Fingerprint: strings.Repeat("cd", 32),
		CreatedAt:   now,
	}
	if err = repo.Create(ctx, duplicateNickname); !errors.Is(err, usersrepoErrors.ErrNicknameTaken) {
		t.Fatalf("duplicate nickname error = %v", err)
	}

	duplicateFingerprint := usersvc.User{
		ID:          uuid.New(),
		Nickname:    "alice2",
		Fingerprint: user.Fingerprint,
		CreatedAt:   now,
	}
	if err = repo.Create(ctx, duplicateFingerprint); !errors.Is(err, usersrepoErrors.ErrFingerprintTaken) {
		t.Fatalf("duplicate fingerprint error = %v", err)
	}

	if _, err = repo.FindByNickname(ctx, "nobody"); !errors.Is(err, usersrepoErrors.ErrNotFound) {
		t.Fatalf("not found error = %v", err)
	}
}

func testChats(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	users := usersrepo.New(pool)
	chats := chatsrepo.New(pool)
	now := time.Now().UTC()

	userIDs := make([]uuid.UUID, 13)
	for i := range userIDs {
		userIDs[i] = uuid.New()
		if err := users.Create(ctx, usersvc.User{
			ID:          userIDs[i],
			Nickname:    fmt.Sprintf("chat_user_%d", i),
			Fingerprint: fmt.Sprintf("%064x", i+1),
			CreatedAt:   now,
		}); err != nil {
			t.Fatalf("create user %d: %v", i, err)
		}
	}

	hostID := userIDs[0]
	chatID := uuid.New()
	token := uuid.New()

	if err := chats.Create(ctx, chatsvc.Chat{
		ID:        chatID,
		HostID:    hostID,
		Token:     token,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create chat: %v", err)
	}

	for _, userID := range userIDs[1:3] {
		if _, err := chats.Join(ctx, token, userID, 4); err != nil {
			t.Fatalf("initial join: %v", err)
		}
	}

	racers := userIDs[3:]
	errs := make(chan error, len(racers))
	winners := make(chan uuid.UUID, len(racers))

	var wg sync.WaitGroup
	for _, userID := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := chats.Join(ctx, token, userID, 4)
			errs <- err
			if err == nil {
				winners <- userID
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(winners)

	var success, full int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, chatsrepoErrors.ErrFull):
			full++
		default:
			t.Fatalf("unexpected join error: %v", err)
		}
	}
	if success != 1 || full != len(racers)-1 {
		t.Fatalf("success=%d full=%d, want 1/%d", success, full, len(racers)-1)
	}

	var winner uuid.UUID
	for userID := range winners {
		winner = userID
	}

	if err := chats.Ban(ctx, chatID, hostID, winner); err != nil {
		t.Fatalf("ban winner: %v", err)
	}
	if _, err := chats.Join(ctx, token, winner, 4); !errors.Is(err, chatsrepoErrors.ErrBanned) {
		t.Fatalf("banned join error = %v", err)
	}
	if err := chats.Delete(ctx, chatID, userIDs[1]); !errors.Is(err, chatsrepoErrors.ErrNotHost) {
		t.Fatalf("non-host delete error = %v", err)
	}
	if err := chats.Delete(ctx, chatID, hostID); err != nil {
		t.Fatalf("host delete: %v", err)
	}
	if member, err := chats.IsMember(ctx, chatID, userIDs[1]); err != nil || member {
		t.Fatalf("is member after delete = %v, err = %v", member, err)
	}
	if _, err := chats.GetByID(ctx, chatID); !errors.Is(err, chatsrepoErrors.ErrNotFound) {
		t.Fatalf("get deleted chat error = %v", err)
	}
}

func testMessages(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	users := usersrepo.New(pool)
	chats := chatsrepo.New(pool)
	messages := messagesrepo.New(pool)
	now := time.Now().UTC()

	userID := uuid.New()
	if err := users.Create(ctx, usersvc.User{
		ID:          userID,
		Nickname:    "msg_user",
		Fingerprint: strings.Repeat("ef", 32),
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	chatID := uuid.New()
	if err := chats.Create(ctx, chatsvc.Chat{
		ID:        chatID,
		HostID:    userID,
		Token:     uuid.New(),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create chat: %v", err)
	}

	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	msgs := []messagesvc.Message{
		{ID: uuid.New(), ChatID: chatID, AuthorID: userID, Text: "first", CreatedAt: base},
		{ID: uuid.New(), ChatID: chatID, AuthorID: userID, Text: "second", CreatedAt: base.Add(time.Second)},
		{ID: uuid.New(), ChatID: chatID, AuthorID: userID, Text: "third", CreatedAt: base.Add(2 * time.Second)},
	}
	for _, msg := range msgs {
		if err := messages.Insert(ctx, msg); err != nil {
			t.Fatalf("insert message: %v", err)
		}
	}

	latest, err := messages.ListLatest(ctx, chatID, 2)
	if err != nil {
		t.Fatalf("list latest: %v", err)
	}
	if len(latest) != 2 || latest[0].Text != "third" || latest[1].Text != "second" {
		t.Fatalf("latest = %+v", latest)
	}

	older, err := messages.ListBefore(ctx, chatID, messagesvc.Cursor{
		CreatedAt: msgs[1].CreatedAt,
		ID:        msgs[1].ID,
	}, 10)
	if err != nil {
		t.Fatalf("list before: %v", err)
	}
	if len(older) != 1 || older[0].Text != "first" {
		t.Fatalf("older = %+v", older)
	}
}
