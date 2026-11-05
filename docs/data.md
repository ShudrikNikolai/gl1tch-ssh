## Модель данных

```sql
users (
    id           uuid PK,
    nickname     text UNIQUE,      -- 3–18, [a-zA-Z0-9_-], неизменяем
    fingerprint  text UNIQUE,      -- hex SHA256 публичного ключа
    created_at   timestamptz
)

chats (
    id          uuid PK,
    host_id     uuid FK users,
    token       uuid UNIQUE,       -- токен приглашения, генерится заново
    created_at  timestamptz,
    deleted_at  timestamptz        -- soft delete
)

chat_members (chat_id, user_id)    -- ≤ 4 на чат, атомарный join
chat_bans    (chat_id, user_id)    -- баны в рамках чата

messages (
    id          uuid PK,
    chat_id     uuid FK chats,
    author_id   uuid FK users,
    text        text,              -- ≤ 300 рун
    created_at  timestamptz
)
```

Ключевые индексы:

- `users(fingerprint)` UNIQUE
- `users(nickname)` UNIQUE
- `chats(token)` UNIQUE — токен генерится заново, не переиспользуется
- `messages(chat_id, created_at DESC, id DESC)` — для пагинации

Вход в комнату выполняется в транзакции с `SELECT ... FOR UPDATE` на строке чата — это гарантирует атомарность проверки лимита и бана при параллельных join'ах.
