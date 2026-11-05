package transport

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name string
		line string
		want command
	}{
		{name: "empty", line: "   ", want: command{name: cmdUnknown}},
		{name: "whoami", line: "/whoami", want: command{name: cmdWhoami}},
		{name: "whoami spaces", line: "  /whoami  ", want: command{name: cmdWhoami}},
		{name: "chats list", line: "/chats-list", want: command{name: cmdChatsList}},
		{name: "chats create", line: "/chats-create", want: command{name: cmdChatsCreate}},
		{name: "chats join", line: "/chats-join token", want: command{name: cmdChatsJoin, arg: "token"}},
		{name: "chats join empty", line: "/chats-join", want: command{name: cmdChatsJoin}},
		{name: "chats delete", line: "/chats-delete", want: command{name: cmdChatsDelete}},
		{name: "unknown", line: "/wat", want: command{name: cmdUnknown}},
		{name: "chat send", line: "@chat /send hello world", want: command{name: cmdSend, text: "hello world", chat: true}},
		{name: "chat send spaces", line: "@chat /send   hello   world  ", want: command{name: cmdSend, text: "hello   world", chat: true}},
		{name: "chat send empty", line: "@chat /send", want: command{name: cmdSend, chat: true}},
		{name: "chat prev", line: "@chat /prev-msg", want: command{name: cmdPrevMsg, chat: true}},
		{name: "chat ban", line: "@chat /ban alice", want: command{name: cmdBan, arg: "alice", chat: true}},
		{name: "chat ban extra", line: "@chat /ban alice extra", want: command{name: cmdBan, arg: "alice", chat: true}},
		{name: "chat bare", line: "@chat", want: command{name: cmdUnknown, chat: true}},
		{name: "chat unknown", line: "@chat /wat", want: command{name: cmdUnknown, chat: true}},
		{name: "chat prefix typo", line: "@chatx /send hi", want: command{name: cmdUnknown}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCommand(tt.line); got != tt.want {
				t.Fatalf("parseCommand(%q) = %+v, want %+v", tt.line, got, tt.want)
			}
		})
	}
}
