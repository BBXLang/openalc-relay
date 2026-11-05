package mailadapter

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// Result-unknown semantics contract (RFC-0003 §4, plan §3.1):
//
//	data stage state                          | returned sentinel
//	------------------------------------------|-----------------------------
//	server refuses MAIL FROM / RCPT / DATA    | ErrSubmitRejected (definite)
//	write fails before the final dot          | plain error (definite failure:
//	                                          | the dot was never sent, so the
//	                                          | server cannot have accepted)
//	final dot answered with 4xx/5xx           | ErrSubmitRejected (definite)
//	final dot reply lost to a dropped         | ErrSubmitUnknown: the server may
//	connection                                | already have accepted; the caller
//	                                          | MUST NOT resubmit automatically
//	final dot answered with 250               | nil (accepted)
//
// The tests below inject a real connection drop after the final dot instead
// of only relying on comments: the fake server speaks enough SMTP over TLS
// to reach the DATA stage and then either drops the connection or answers
// the dot. All of this runs on 127.0.0.1 with synthetic credentials.

// fakeSMTPBehavior describes what the fake submission endpoint does around
// the DATA stage.
type fakeSMTPBehavior struct {
	mailReply  string // answer to MAIL FROM; default "250 ok"
	rcptReply  string // answer to RCPT TO; default "250 ok"
	dataReply  string // answer after the final dot; default "250 accepted"
	dropAtData bool   // close the connection after the final dot, no reply
}

// fakeSMTPServer is a minimal scriptable SMTP-over-TLS endpoint.
type fakeSMTPServer struct {
	listener net.Listener
	RootCAs  *x509.CertPool
	port     int
}

func startFakeSMTPServer(t *testing.T, behavior fakeSMTPBehavior) fakeSMTPServer {
	t.Helper()
	cert := selfSignedCert(t)
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	pool := x509.NewCertPool()
	pool.AddCert(cert.Leaf)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		tlsConn := tls.Server(conn, serverTLS)
		if err := tlsConn.Handshake(); err != nil {
			return
		}
		serveFakeSession(tlsConn, behavior)
	}()
	t.Cleanup(func() { listener.Close() })
	return fakeSMTPServer{listener: listener, RootCAs: pool, port: listener.Addr().(*net.TCPAddr).Port}
}

func serveFakeSession(conn net.Conn, behavior fakeSMTPBehavior) {
	reader := bufio.NewReader(conn)
	write := func(s string) {
		_, _ = io.WriteString(conn, s)
	}
	write("220 fake.example.invalid ESMTP\r\n")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.ToUpper(strings.TrimRight(line, "\r\n"))
		switch {
		case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
			write("250-fake.example.invalid\r\n250-AUTH PLAIN\r\n250 8BITMIME\r\n")
		case strings.HasPrefix(command, "AUTH"):
			write("235 ok\r\n")
		case strings.HasPrefix(command, "MAIL FROM"):
			reply := behavior.mailReply
			if reply == "" {
				reply = "250 ok"
			}
			write(reply + "\r\n")
		case strings.HasPrefix(command, "RCPT TO"):
			reply := behavior.rcptReply
			if reply == "" {
				reply = "250 ok"
			}
			write(reply + "\r\n")
		case command == "DATA":
			write("354 go ahead\r\n")
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
			}
			if behavior.dropAtData {
				// The final dot was received but the acceptance reply is
				// lost: exactly the "result unknown" window.
				return
			}
			reply := behavior.dataReply
			if reply == "" {
				reply = "250 accepted"
			}
			write(reply + "\r\n")
		case strings.HasPrefix(command, "QUIT"):
			write("221 bye\r\n")
			return
		default:
			write("502 not implemented\r\n")
		}
	}
}

func fakeSubmissionConfig(server fakeSMTPServer) SMTPConfig {
	return SMTPConfig{
		Host: "127.0.0.1", Port: server.port, TLS: TLSImplicit,
		Username: syntheticSMTPUser, AppPassword: syntheticSMTPPass,
		RootCAs: server.RootCAs,
		Timeout: 10 * time.Second,
	}
}

func fakeSubmission(messageID string) Submission {
	return Submission{
		From:      "sender@example.invalid",
		To:        []string{"user@example.invalid"},
		MessageID: messageID,
		Data: syntheticMessage(messageID, "Result unknown probe",
			"plain", "<p>html</p>"),
	}
}

// TestSubmitUnknownWhenReplyIsLost injects a connection drop after the final
// dot: the adapter must report ErrSubmitUnknown, never a definite failure,
// so the upper layer marks the send intent as "result unknown" and refuses
// to resubmit automatically.
func TestSubmitUnknownWhenReplyIsLost(t *testing.T) {
	server := startFakeSMTPServer(t, fakeSMTPBehavior{dropAtData: true})
	err := Submit(context.Background(), fakeSubmissionConfig(server), loopbackPolicy(server.port),
		fakeSubmission("unknown-0001@example.invalid"))
	if !errors.Is(err, ErrSubmitUnknown) {
		t.Fatalf("got %v, want ErrSubmitUnknown", err)
	}
	if errors.Is(err, ErrSubmitRejected) {
		t.Fatalf("unknown result must not be classified as rejected: %v", err)
	}
}

// TestSubmitRejectedAfterData injects an explicit 554 after the final dot:
// definite failure, not "unknown".
func TestSubmitRejectedAfterData(t *testing.T) {
	server := startFakeSMTPServer(t, fakeSMTPBehavior{dataReply: "554 message rejected"})
	err := Submit(context.Background(), fakeSubmissionConfig(server), loopbackPolicy(server.port),
		fakeSubmission("rejected-0001@example.invalid"))
	if !errors.Is(err, ErrSubmitRejected) {
		t.Fatalf("got %v, want ErrSubmitRejected", err)
	}
	if errors.Is(err, ErrSubmitUnknown) {
		t.Fatalf("explicit rejection must not be classified as unknown: %v", err)
	}
}

// TestSubmitRejectedAtMailFrom injects a 550 at MAIL FROM: the message was
// never accepted and never could have been.
func TestSubmitRejectedAtMailFrom(t *testing.T) {
	server := startFakeSMTPServer(t, fakeSMTPBehavior{mailReply: "550 sender blocked"})
	err := Submit(context.Background(), fakeSubmissionConfig(server), loopbackPolicy(server.port),
		fakeSubmission("mailfrom-0001@example.invalid"))
	if !errors.Is(err, ErrSubmitRejected) {
		t.Fatalf("got %v, want ErrSubmitRejected", err)
	}
	if errors.Is(err, ErrSubmitUnknown) {
		t.Fatalf("pre-DATA failure must not be classified as unknown: %v", err)
	}
}

// TestSubmitAcceptedOverRealServerRoundTrip pins the success path against
// the full go-smtp server (already covered end-to-end in the e2e test) and
// verifies the adapter accepts exactly one submission attempt.
func TestSubmitAcceptedOnce(t *testing.T) {
	servers := startMailServers(t, TLSImplicit)
	err := Submit(context.Background(), SMTPConfig{
		Host: "127.0.0.1", Port: servers.SMTPPort, TLS: TLSImplicit,
		Username: syntheticSMTPUser, AppPassword: syntheticSMTPPass,
		RootCAs: servers.RootCAs,
	}, loopbackPolicy(servers.SMTPPort), fakeSubmission("accepted-0001@example.invalid"))
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	servers.Backend.captured(t) // exactly one message captured
}

// TestSubmitValidatesBeforeNetwork proves the adapter refuses invalid
// submissions and mismatched Message-IDs before touching the network.
func TestSubmitValidatesBeforeNetwork(t *testing.T) {
	cases := []struct {
		name    string
		submit  Submission
		wantErr error
	}{
		{"missing message id", Submission{From: "sender@example.invalid",
			To: []string{"user@example.invalid"}, MessageID: "", Data: []byte("x\r\n")},
			ErrSubmissionInvalid},
		{"message id mismatch", Submission{From: "sender@example.invalid",
			To: []string{"user@example.invalid"}, MessageID: "<one@example.invalid>",
			Data: []byte("Message-ID: <other@example.invalid>\r\n\r\nbody\r\n")},
			ErrSubmissionInvalid},
		{"empty recipients", Submission{From: "sender@example.invalid",
			To: nil, MessageID: "<one@example.invalid>",
			Data: []byte("Message-ID: <one@example.invalid>\r\n\r\nbody\r\n")},
			ErrSubmissionInvalid},
		{"bad sender", Submission{From: "not-an-address",
			To: []string{"user@example.invalid"}, MessageID: "<one@example.invalid>",
			Data: []byte("Message-ID: <one@example.invalid>\r\n\r\nbody\r\n")},
			ErrSubmissionInvalid},
	}
	for _, tc := range cases {
		// Nothing listens on the port; the validation errors must fire
		// before the dial is even attempted.
		err := Submit(context.Background(), SMTPConfig{
			Host: "127.0.0.1", Port: 9, TLS: TLSImplicit,
			Username: "synthetic", AppPassword: "synthetic",
		}, EndpointPolicy{}, tc.submit)
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.wantErr)
		}
	}
}

// TestConfigValidation covers the TLS-mode and credential rules: plaintext
// is refused by default for both protocols, unknown modes are refused, and
// missing credentials are refused.
func TestConfigValidation(t *testing.T) {
	valid := IMAPConfig{Host: "mail.example.invalid", Port: 993, TLS: TLSImplicit,
		Username: "user", AppPassword: "pass"}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid IMAP config: %v", err)
	}
	for _, tc := range []struct {
		name    string
		mutate  func(*IMAPConfig)
		wantErr error
	}{
		{"plaintext empty TLS", func(c *IMAPConfig) { c.TLS = "" }, ErrConfigInvalid},
		{"unknown TLS mode", func(c *IMAPConfig) { c.TLS = TLSPreference("tls") }, ErrConfigInvalid},
		{"missing username", func(c *IMAPConfig) { c.Username = "" }, ErrCredentialsMissing},
		{"missing app password", func(c *IMAPConfig) { c.AppPassword = "" }, ErrCredentialsMissing},
		{"missing host", func(c *IMAPConfig) { c.Host = "" }, ErrConfigInvalid},
		{"port out of range", func(c *IMAPConfig) { c.Port = 70000 }, ErrConfigInvalid},
	} {
		cfg := valid
		tc.mutate(&cfg)
		err := cfg.validate()
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("imap %s: got %v, want %v", tc.name, err, tc.wantErr)
		}
	}
	validSMTP := SMTPConfig{Host: "mail.example.invalid", Port: 465, TLS: TLSStartTLS,
		Username: "user", AppPassword: "pass"}
	if err := validSMTP.validate(); err != nil {
		t.Fatalf("valid SMTP config: %v", err)
	}
	err := (SMTPConfig{Host: "mail.example.invalid", Port: 587, TLS: "plain",
		Username: "u", AppPassword: "p"}).validate()
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("smtp plaintext mode: got %v, want ErrConfigInvalid", err)
	}
}

// TestIMAPDialRefusesPlaintextMode proves the TLS gate fires before any
// connection attempt: port 1 has no listener, so reaching the dial at all
// would surface as a connection error, not ErrConfigInvalid.
func TestIMAPDialRefusesPlaintextMode(t *testing.T) {
	_, err := Dial(context.Background(), IMAPConfig{
		Host: "127.0.0.1", Port: 1, TLS: TLSPreference(""),
		Username: "synthetic", AppPassword: "synthetic",
	}, EndpointPolicy{})
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("got %v, want ErrConfigInvalid", err)
	}
}

// TestTLSConfigNeverSkipsVerification pins RFC-0003 §5: the adapter never
// offers a way to skip certificate or hostname verification.
func TestTLSConfigNeverSkipsVerification(t *testing.T) {
	cfg := tlsConfig("mail.example.invalid", nil)
	if cfg.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify must never be set")
	}
	if cfg.ServerName != "mail.example.invalid" {
		t.Fatalf("ServerName %q", cfg.ServerName)
	}
	if cfg.MinVersion < tls.VersionTLS12 {
		t.Fatalf("MinVersion %#x below TLS 1.2", cfg.MinVersion)
	}
}
