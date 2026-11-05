package transport

import "strings"

const (
	cmdUnknown = "unknown"

	cmdWhoami      = "whoami"
	cmdChatsList   = "chats-list"
	cmdChatsCreate = "chats-create"
	cmdChatsJoin   = "chats-join"
	cmdChatsDelete = "chats-delete"

	cmdSend    = "send"
	cmdPrevMsg = "prev-msg"
	cmdBan     = "ban"
)

type command struct {
	name string
	arg  string
	text string
	chat bool
}

func parseCommand(line string) command {
	line = strings.TrimSpace(line)
	if line == "" {
		return command{name: cmdUnknown}
	}

	if line == "@chat" || strings.HasPrefix(line, "@chat ") {
		return parseChatCommand(strings.TrimSpace(strings.TrimPrefix(line, "@chat")))
	}

	name, args, _ := strings.Cut(line, " ")
	args = strings.TrimSpace(args)

	switch name {
	case "/whoami":
		return command{name: cmdWhoami}
	case "/chats-list":
		return command{name: cmdChatsList}
	case "/chats-create":
		return command{name: cmdChatsCreate}
	case "/chats-join":
		return command{name: cmdChatsJoin, arg: args}
	case "/chats-delete":
		return command{name: cmdChatsDelete}
	default:
		return command{name: cmdUnknown}
	}
}

func parseChatCommand(line string) command {
	name, args, _ := strings.Cut(line, " ")
	args = strings.TrimSpace(args)

	switch name {
	case "/send":
		return command{name: cmdSend, text: args, chat: true}
	case "/prev-msg":
		return command{name: cmdPrevMsg, chat: true}
	case "/ban":
		fields := strings.Fields(args)
		nickname := ""
		if len(fields) > 0 {
			nickname = fields[0]
		}

		return command{name: cmdBan, arg: nickname, chat: true}
	default:
		return command{name: cmdUnknown, chat: true}
	}
}
