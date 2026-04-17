package email_test

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"ledger/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startMockSMTP starts a minimal SMTP server on a random localhost port.
// It captures the DATA payload and returns a function to retrieve it.
// smtp.PlainAuth allows plaintext auth when the host is 127.0.0.1.
func startMockSMTP(t *testing.T) (host, port string, body func() string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	var mu sync.Mutex
	var captured strings.Builder

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		_, _ = fmt.Fprintf(conn, "220 localhost SMTP\r\n")

		inData := false
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text() // \r stripped by ScanLines

			if inData {
				if line == "." {
					_, _ = fmt.Fprintf(conn, "250 OK\r\n")
					inData = false
				} else {
					mu.Lock()
					captured.WriteString(line + "\n")
					mu.Unlock()
				}
				continue
			}

			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				// Advertise AUTH PLAIN so smtp.PlainAuth proceeds.
				_, _ = fmt.Fprintf(conn, "250-localhost\r\n250 AUTH PLAIN LOGIN\r\n")
			case strings.HasPrefix(upper, "AUTH"):
				_, _ = fmt.Fprintf(conn, "235 OK\r\n")
			case strings.HasPrefix(upper, "MAIL"):
				_, _ = fmt.Fprintf(conn, "250 OK\r\n")
			case strings.HasPrefix(upper, "RCPT"):
				_, _ = fmt.Fprintf(conn, "250 OK\r\n")
			case upper == "DATA":
				_, _ = fmt.Fprintf(conn, "354 Start mail input\r\n")
				inData = true
			case strings.HasPrefix(upper, "QUIT"):
				_, _ = fmt.Fprintf(conn, "221 Bye\r\n")
				return
			}
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", strconv.Itoa(addr.Port), func() string {
		mu.Lock()
		defer mu.Unlock()
		return captured.String()
	}
}

func TestSendVerificationEmail(t *testing.T) {
	host, port, getBody := startMockSMTP(t)

	t.Setenv("SMTP_HOST", host)
	t.Setenv("SMTP_PORT", port)
	t.Setenv("SMTP_USER", "user")
	t.Setenv("SMTP_PASS", "pass")
	t.Setenv("SMTP_FROM", "noreply@example.com")
	t.Setenv("APP_URL", "https://app.example.com")

	err := email.SendVerificationEmail("invited@example.com", "jsmith", "abc123token")
	require.NoError(t, err)

	body := getBody()
	assert.Contains(t, body, "https://app.example.com/verify?token=abc123token")
	assert.Contains(t, body, "jsmith")
	assert.Contains(t, body, "invited@example.com")
}

func TestSendVerificationEmail_TokenInURL(t *testing.T) {
	host, port, getBody := startMockSMTP(t)

	t.Setenv("SMTP_HOST", host)
	t.Setenv("SMTP_PORT", port)
	t.Setenv("SMTP_USER", "u")
	t.Setenv("SMTP_PASS", "p")
	t.Setenv("SMTP_FROM", "from@example.com")
	t.Setenv("APP_URL", "http://localhost:3000")

	require.NoError(t, email.SendVerificationEmail("to@example.com", "alice", "deadbeef"))

	assert.Contains(t, getBody(), "http://localhost:3000/verify?token=deadbeef")
}
