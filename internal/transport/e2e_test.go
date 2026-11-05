package transport_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
)

var tokenPattern = regexp.MustCompile(`Token: ([0-9a-fA-F-]{36})`)

type testClient struct {
	t       *testing.T
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	mu      sync.Mutex
	output  strings.Builder
}

func newTestClient(t *testing.T, addr string) *testClient {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "e2e",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}

	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		t.Fatalf("new session: %v", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		t.Fatalf("stdin: %v", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		t.Fatalf("stdout: %v", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		t.Fatalf("stderr: %v", err)
	}

	c := &testClient{
		t:       t,
		client:  client,
		session: session,
		stdin:   stdin,
	}

	go c.consume(stdout)
	go c.consume(stderr)

	if err = session.RequestPty("xterm", 40, 120, ssh.TerminalModes{}); err != nil {
		c.close()
		t.Fatalf("request pty: %v", err)
	}

	if err = session.Shell(); err != nil {
		c.close()
		t.Fatalf("shell: %v", err)
	}

	return c
}

func (c *testClient) consume(reader io.Reader) {
	buffer := make([]byte, 4096)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			c.mu.Lock()
			c.output.Write(buffer[:n])
			c.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (c *testClient) outputString() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.output.String()
}

func (c *testClient) waitFor(substring string) string {
	c.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out := c.outputString()
		if strings.Contains(out, substring) {
			return out
		}

		time.Sleep(10 * time.Millisecond)
	}

	c.t.Fatalf("timed out waiting for %q; output:\n%s", substring, c.outputString())

	return ""
}

func (c *testClient) waitForToken() string {
	c.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if match := tokenPattern.FindStringSubmatch(c.outputString()); match != nil {
			return match[1]
		}

		time.Sleep(10 * time.Millisecond)
	}

	c.t.Fatalf("timed out waiting for chat token; output:\n%s", c.outputString())

	return ""
}

func (c *testClient) send(line string) {
	c.t.Helper()

	// Интерактивный Enter в PTY приходит как \r, а не \n.
	if _, err := io.WriteString(c.stdin, line+"\r"); err != nil {
		c.t.Fatalf("send %q: %v", line, err)
	}
}

func (c *testClient) close() {
	_ = c.session.Close()
	_ = c.client.Close()
}

func TestE2EChatFlow(t *testing.T) {
	addr := os.Getenv("E2E_ADDR")
	if addr == "" {
		t.Skip("E2E_ADDR is not set; start the server and set E2E_ADDR=localhost:2222")
	}

	_ = godotenv.Load("../../.env")

	suffix := time.Now().UnixNano() % 1_000_000_000
	aliceNick := fmt.Sprintf("alice%d", suffix)
	bobNick := fmt.Sprintf("bob%d", suffix)

	if dsn := os.Getenv("PG_DSN"); dsn != "" {
		t.Cleanup(func() { cleanupE2EUsers(dsn, aliceNick, bobNick) })
	}

	alice := newTestClient(t, addr)
	defer alice.close()

	alice.waitFor("Choose a nickname")
	alice.send(aliceNick)
	alice.waitFor("Welcome, " + aliceNick)

	alice.send("/whoami")
	alice.waitFor("You are " + aliceNick)

	alice.send("/chats-create")
	token := alice.waitForToken()

	bob := newTestClient(t, addr)
	defer bob.close()

	bob.waitFor("Choose a nickname")
	bob.send(bobNick)
	bob.waitFor("Welcome, " + bobNick)

	bob.send("/chats-join " + token)
	bob.waitFor("Joined chat")

	bob.send("@chat /send hello from bob")
	bob.waitFor(bobNick + ": hello from bob")
	alice.waitFor(bobNick + ": hello from bob")

	alice.send("@chat /send hello from alice")
	alice.waitFor(aliceNick + ": hello from alice")
	bob.waitFor(aliceNick + ": hello from alice")

	bob.send("@chat /prev-msg")
	bob.waitFor("hello from bob")
	bob.waitFor("hello from alice")

	alice.send("/chats-list")
	alice.waitFor(token)

	bob.send("/chats-delete")
	bob.waitFor("Only the chat host can do that.")

	alice.send("@chat /ban " + bobNick)
	alice.waitFor("Banned " + bobNick + ".")

	alice.send("@chat /send after ban")
	bob.waitFor("You are not in a chat. Disconnecting.")

	alice.send("/chats-delete")
	alice.waitFor("Chat deleted.")
}

func cleanupE2EUsers(dsn string, nicknames ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return
	}
	defer pool.Close()

	_, _ = pool.Exec(
		ctx,
		`DELETE FROM chats WHERE host_id IN (SELECT id FROM users WHERE nickname = ANY($1))`,
		nicknames,
	)
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE nickname = ANY($1)`, nicknames)
}
