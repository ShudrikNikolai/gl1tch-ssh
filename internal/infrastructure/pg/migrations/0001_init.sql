-- +goose Up
CREATE TABLE users (
                       id           uuid PRIMARY KEY,
                       nickname     text NOT NULL UNIQUE,
                       fingerprint  text NOT NULL UNIQUE,  -- hex SHA256
                       created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE chats (
                       id          uuid PRIMARY KEY,           -- uuidv7
                       host_id     uuid NOT NULL REFERENCES users(id),
                       token       uuid NOT NULL UNIQUE,
                       created_at  timestamptz NOT NULL DEFAULT now(),
                       deleted_at  timestamptz
);

CREATE TABLE chat_members (
                              chat_id  uuid NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
                              user_id  uuid NOT NULL REFERENCES users(id),
                              joined_at timestamptz NOT NULL DEFAULT now(),
                              PRIMARY KEY (chat_id, user_id)
);
CREATE INDEX idx_chat_members_user ON chat_members(user_id); -- для /chats-list

CREATE TABLE chat_bans (
                           chat_id  uuid NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
                           user_id  uuid NOT NULL REFERENCES users(id),
                           banned_at timestamptz NOT NULL DEFAULT now(),
                           PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE messages (
                          id         uuid PRIMARY KEY,
                          chat_id    uuid NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
                          author_id  uuid NOT NULL REFERENCES users(id),
                          text       text NOT NULL,
                          created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_chat_created ON messages(chat_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chat_bans;
DROP TABLE IF EXISTS chat_members;
DROP TABLE IF EXISTS chats;
DROP TABLE IF EXISTS users;
