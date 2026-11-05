package transport

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func newInputSession(input string, pty bool) (*session, *bytes.Buffer) {
	buffer := &bytes.Buffer{}

	return &session{
		reader: bufio.NewReader(strings.NewReader(input)),
		pty:    pty,
		out:    &sessionWriter{writer: buffer},
	}, buffer
}

func TestReadInputLineEndings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "lf", input: "alice\nbob\n", want: []string{"alice", "bob"}},
		{name: "cr", input: "alice\rbob\r", want: []string{"alice", "bob"}},
		{name: "crlf", input: "alice\r\nbob\r\n", want: []string{"alice", "bob"}},
		{name: "mixed", input: "alice\r\nbob\ncarol\r", want: []string{"alice", "bob", "carol"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, _ := newInputSession(tt.input, false)

			for _, want := range tt.want {
				got, err := session.readInputLine()
				if err != nil {
					t.Fatalf("read line: %v", err)
				}
				if got != want {
					t.Fatalf("line = %q, want %q", got, want)
				}
			}

			if _, err := session.readInputLine(); !errors.Is(err, io.EOF) {
				t.Fatalf("final error = %v, want EOF", err)
			}
		})
	}
}

func TestReadInputLineEditing(t *testing.T) {
	t.Run("backspace", func(t *testing.T) {
		session, _ := newInputSession("aliX\x7fce\r", true)

		got, err := session.readInputLine()
		if err != nil {
			t.Fatalf("read line: %v", err)
		}
		if got != "alice" {
			t.Fatalf("line = %q, want alice", got)
		}
	})

	t.Run("unicode", func(t *testing.T) {
		session, _ := newInputSession("привет\r", false)

		got, err := session.readInputLine()
		if err != nil {
			t.Fatalf("read line: %v", err)
		}
		if got != "привет" {
			t.Fatalf("line = %q, want привет", got)
		}
	})

	t.Run("ctrl-c", func(t *testing.T) {
		session, _ := newInputSession("alice\x03", true)

		if _, err := session.readInputLine(); !errors.Is(err, io.EOF) {
			t.Fatalf("error = %v, want EOF", err)
		}
	})

	t.Run("ctrl-d empty", func(t *testing.T) {
		session, _ := newInputSession("\x04", true)

		if _, err := session.readInputLine(); !errors.Is(err, io.EOF) {
			t.Fatalf("error = %v, want EOF", err)
		}
	})
}

func TestReadInputLineEcho(t *testing.T) {
	t.Run("echoes in pty", func(t *testing.T) {
		session, buffer := newInputSession("alice\r", true)

		if _, err := session.readInputLine(); err != nil {
			t.Fatalf("read line: %v", err)
		}
		if buffer.String() != "alice\n" {
			t.Fatalf("echo = %q, want %q", buffer.String(), "alice\n")
		}
	})

	t.Run("does not echo without pty", func(t *testing.T) {
		session, buffer := newInputSession("alice\r", false)

		if _, err := session.readInputLine(); err != nil {
			t.Fatalf("read line: %v", err)
		}
		if buffer.Len() != 0 {
			t.Fatalf("echo = %q, want empty", buffer.String())
		}
	})
}
