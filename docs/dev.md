## Разработка

```bash
task lint               # golangci-lint
task fmt                # gofumpt + gci
task vet                # go vet ./...
task test               # go test ./...
task test:race          # с -race
task test:integration   # интеграционные тесты (нужен Docker)
task test:e2e           # SSH E2E против запущенного сервера (E2E_ADDR)
task build              # бинарник в ./bin
task migrate:up         # применить миграции (обычно не нужно: сервер делает это сам)
task migrate:down       # откатить последнюю
task migrate:status     # статус миграций
```

Миграции — goose, файлы `internal/infrastructure/pg/migrations/NNNN_name.sql`.
Версия в имени файла должна быть ≥ 1: `0000_*.sql` goose игнорирует.
Таблица миграций — `schema_migrations` (CLI-задача уже передаёт её через `-table`).

На Windows `task test:race` требует `CGO_ENABLED=1` и установленный gcc; без них используйте `task test` или WSL/Docker.

### Структура модуля

Каждый доменный модуль (`chats`, `messages`, `users`) следует одной схеме:

```
modules/<name>/
  repository/
    repository.go     — интерфейс
    models/           — DB-модели
    converter/        — domain ↔ DB
    repo/
      client.go       — pgxpool/querier
      repo.go         — реализация интерфейса
  service/
    service.go        — интерфейс
    converter.go      — domain ↔ transport
    model.go          — доменные модели
    srv/              — реализация
```

Модуль `aggregate` — для сквозных сценариев, которым нужны данные из нескольких модулей одновременно (например, «последние 20 сообщений с никами авторов»). Бизнес-логики в нём минимум, только оркестрация вызовов сервисов.

### Тестирование

- Юнит-тесты сервисов — с моками репозиториев (интерфейсы в `repository/repository.go`).
- Интеграционные тесты репозиториев — против `testcontainers-go` с Postgres, под build-тегом `integration` (`internal/integration`).
- Тест `Broadcaster` — против реального Redis (testcontainers) или `miniredis`.
- E2E — клиент `golang.org/x/crypto/ssh` к запущенному серверу. Запуск: `task run` в одном терминале, затем `E2E_ADDR=localhost:2222 task test:e2e`.

