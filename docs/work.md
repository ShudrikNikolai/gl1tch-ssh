## Пример работы

Ниже — сценарий на двух клиентах. Каждый клиент подключается **своим SSH-ключом**: fingerprint ключа — это и есть аккаунт.

Подготовка:

```bash
task deploy:up
task run
```

### Клиент 1 — alice

```bash
ssh -p 2222 localhost
# если host-key сервера изменился:
# ssh -o StrictHostKeyChecking=accept-new -p 2222 localhost
```

Первый вход: ник выбирается один раз и больше не меняется. Выйти из сессии можно по `Ctrl+D` или `Ctrl+C`.

```text
gl1tch-ssh
Choose a nickname (3-18 chars, A-Z a-z 0-9 _ -): alice
Welcome, alice!
> /whoami
You are alice.
> /chats-create
Chat created.
Token: 019a7f3c-6f1a-7a2b-9c4d-2f6e8a1b3c5d
> @chat /send привет, это alice
[12:34] alice: привет, это alice
```

Токен из строки `Token:` — приглашение в комнату. Он генерируется заново для каждой новой комнаты и не переиспользуется.

### Клиент 2 — bob

```bash
ssh -p 2222 localhost
```

```text
gl1tch-ssh
Choose a nickname (3-18 chars, A-Z a-z 0-9 _ -): bob
Welcome, bob!
> /chats-join 019a7f3c-6f1a-7a2b-9c4d-2f6e8a1b3c5d
Joined chat 019a7f3c-6f1a-7a2b-9c4d-2f6e8a1b3c5d.
[12:34] alice: привет, это alice
> @chat /send привет, это bob
[12:35] bob: привет, это bob
```

alice в этот момент видит сообщение bob в реальном времени:

```text
[12:35] bob: привет, это bob
```

История и список комнат:

```text
> @chat /prev-msg
[12:34] alice: привет, это alice
[12:35] bob: привет, это bob

> /chats-list
- 019a7f3c-6f1a-7a2b-9c4d-2f6e8a1b3c5d token=019a7f3c-... (host) created=2026-11-05 12:34
```

### Бан и удаление

Bob не является хостом, поэтому удалить комнату не может:

```text
> /chats-delete
Only the chat host can do that.
```

alice банит bob:

```text
> @chat /ban bob
Banned bob.
> @chat /send после бана
[12:36] alice: после бана
```

Сессия bob получает следующее сообщение и отключается, потому что он больше не участник:

```text
You are not in a chat. Disconnecting.
```

alice удаляет комнату:

```text
> /chats-delete
Chat deleted.
```

Сообщения самого отправителя тоже приходят через Redis Pub/Sub — локального эха нет. Если Redis недоступен, сообщение всё равно сохранится в истории, но live-доставка не сработает.
