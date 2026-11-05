package mailadapter

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// All credentials in this file are synthetic (example.invalid domain) and all
// endpoints are in-process servers on 127.0.0.1 with random ports. Nothing
// here contacts a real mailbox, an external host, or a real mail provider.

const (
	syntheticIMAPUser = "sync@example.invalid"
	syntheticIMAPPass = "synthetic-imap-app-password"
	syntheticSMTPUser = "submit@example.invalid"
	syntheticSMTPPass = "synthetic-smtp-app-password"
)

// loopbackPolicy is the policy used by the tests: it is the explicit
// administrator allowlist that lets the adapter reach the in-process
// controlled endpoints on 127.0.0.1 (loopback, on exactly the given random
// test ports), which are denied by default.
func loopbackPolicy(ports ...int) EndpointPolicy {
	allowed := make(map[int]bool, len(ports))
	for _, port := range ports {
		allowed[port] = true
	}
	return EndpointPolicy{
		PrivateAllow: func(kind ServiceKind, host string, ip net.IP, port int) bool {
			return ip.IsLoopback() && allowed[port]
		},
	}
}

// selfSignedCert generates a throwaway server certificate valid for
// 127.0.0.1/localhost. The tests hand its public half to the adapter's
// RootCAs trust store; verification itself stays fully enabled.
func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

// memoryBackend is the go-smtp backend capturing submitted messages so the
// test can seed the controlled IMAP mailbox with them.
type memoryBackend struct {
	mu       sync.Mutex
	messages [][]byte
}

func (b *memoryBackend) NewSession(*smtp.Conn) (smtp.Session, error) {
	return &captureSession{backend: b}, nil
}

func (b *memoryBackend) captured(t *testing.T) []byte {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.messages) != 1 {
		t.Fatalf("captured %d submissions, want 1", len(b.messages))
	}
	return b.messages[0]
}

type captureSession struct {
	backend *memoryBackend
}

func (s *captureSession) Reset()                               {}
func (s *captureSession) Logout() error                        { return nil }
func (s *captureSession) Mail(string, *smtp.MailOptions) error { return nil }
func (s *captureSession) Rcpt(string, *smtp.RcptOptions) error { return nil }

func (s *captureSession) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	s.backend.messages = append(s.backend.messages, data)
	return nil
}

// Auth verifies the synthetic submission credentials; this is what makes the
// in-process endpoint a controlled stand-in for a real submission service.
func (s *captureSession) AuthMechanisms() []string { return []string{"PLAIN"} }

func (s *captureSession) Auth(mech string) (sasl.Server, error) {
	if mech != "PLAIN" {
		return nil, errors.New("unsupported mechanism")
	}
	return sasl.NewPlainServer(func(_, username, password string) error {
		if username != syntheticSMTPUser || password != syntheticSMTPPass {
			return errors.New("invalid credentials")
		}
		return nil
	}), nil
}

// literalReader adapts a byte slice to the IMAP literal reader used to seed
// the in-process mailbox.
type literalReader struct {
	*bytes.Reader
	size int64
}

func (l literalReader) Size() int64 { return l.size }

// mailServers holds the two in-process controlled endpoints.
type mailServers struct {
	SMTPPort int
	IMAPPort int
	RootCAs  *x509.CertPool
	Backend  *memoryBackend
	User     *imapmemserver.User
}

// startMailServers launches an in-process SMTP submission server and an
// in-process IMAP server (both on 127.0.0.1 random ports, TLS per mode) and
// registers their shutdown with the test.
func startMailServers(t *testing.T, mode TLSPreference) mailServers {
	t.Helper()
	cert := selfSignedCert(t)
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	pool := x509.NewCertPool()
	pool.AddCert(cert.Leaf)

	backend := &memoryBackend{}
	smtpServer := smtp.NewServer(backend)
	smtpServer.Domain = "mail.example.invalid"
	smtpServer.MaxMessageBytes = 1 << 20
	smtpServer.MaxRecipients = 10
	smtpServer.TLSConfig = serverTLS // enables STARTTLS advertisement
	smtpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if mode == TLSImplicit {
		smtpListener = tls.NewListener(smtpListener, serverTLS)
	}
	go smtpServer.Serve(smtpListener)

	memory := imapmemserver.New()
	user := imapmemserver.NewUser(syntheticIMAPUser, syntheticIMAPPass)
	memory.AddUser(user)
	if err := user.Create("INBOX", nil); err != nil {
		t.Fatal(err)
	}
	imapServer := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memory.NewSession(), nil, nil
		},
		TLSConfig: serverTLS, // enables STARTTLS advertisement
	})
	imapListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if mode == TLSImplicit {
		imapListener = tls.NewListener(imapListener, serverTLS)
	}
	go imapServer.Serve(imapListener)

	t.Cleanup(func() {
		smtpServer.Close()
		imapServer.Close()
	})
	return mailServers{
		SMTPPort: smtpListener.Addr().(*net.TCPAddr).Port,
		IMAPPort: imapListener.Addr().(*net.TCPAddr).Port,
		RootCAs:  pool,
		Backend:  backend,
		User:     user,
	}
}

// syntheticMessage builds a deterministic multipart/alternative RFC 5322
// message. The HTML part deliberately contains a script tag: ParseMIME must
// return it verbatim because sanitization belongs to the upper layer.
func syntheticMessage(messageID, subject, text, html string) []byte {
	var b strings.Builder
	b.WriteString("From: Synthetic Sender <sender@example.invalid>\r\n")
	b.WriteString("To: Relay User <user@example.invalid>, other@example.invalid\r\n")
	b.WriteString("Cc: cc-one@example.invalid\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("Date: Fri, 06 Nov 2026 09:30:00 +0000\r\n")
	b.WriteString("Message-ID: <" + messageID + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"batch3-alt\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("--batch3-alt\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(text + "\r\n")
	b.WriteString("--batch3-alt\r\n")
	b.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(html + "\r\n")
	b.WriteString("--batch3-alt--\r\n")
	return []byte(b.String())
}

// TestEndToEndSubmitFetchParse covers the full controlled chain: submit one
// synthetic message to the in-process SMTP endpoint, seed the in-process
// IMAP mailbox with the captured bytes, FETCH it back through the adapter
// and parse subject, sender and bodies out of the raw MIME.
func TestEndToEndSubmitFetchParse(t *testing.T) {
	const (
		messageID = "batch3-e2e-0001@example.invalid"
		subject   = "Batch 3 synthetic ping"
		text      = "Hello batch three."
		html      = "<p>Hello <b>batch</b> three.<script>alert(1)</script></p>"
	)
	for _, mode := range []TLSPreference{TLSImplicit, TLSStartTLS} {
		t.Run(string(mode), func(t *testing.T) {
			servers := startMailServers(t, mode)
			ctx := context.Background()

			data := syntheticMessage(messageID, subject, text, html)
			err := Submit(ctx, SMTPConfig{
				Host: "127.0.0.1", Port: servers.SMTPPort, TLS: mode,
				Username: syntheticSMTPUser, AppPassword: syntheticSMTPPass,
				RootCAs: servers.RootCAs,
			}, loopbackPolicy(servers.SMTPPort), Submission{
				From:      "sender@example.invalid",
				To:        []string{"user@example.invalid"},
				MessageID: messageID,
				Data:      data,
			})
			if err != nil {
				t.Fatalf("smtp submit: %v", err)
			}

			captured := servers.Backend.captured(t)
			if !bytes.Equal(captured, data) {
				t.Fatalf("captured message differs from submitted data")
			}
			_, err = servers.User.Append("INBOX", literalReader{
				Reader: bytes.NewReader(captured),
				size:   int64(len(captured)),
			}, &imap.AppendOptions{Time: time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)})
			if err != nil {
				t.Fatalf("seed imap mailbox: %v", err)
			}

			client, err := Dial(ctx, IMAPConfig{
				Host: "127.0.0.1", Port: servers.IMAPPort, TLS: mode,
				Username: syntheticIMAPUser, AppPassword: syntheticIMAPPass,
				RootCAs: servers.RootCAs,
			}, loopbackPolicy(servers.IMAPPort))
			if err != nil {
				t.Fatalf("imap dial: %v", err)
			}
			defer client.Close()

			snapshot := client.Snapshot()
			if snapshot.UIDValidity == 0 || snapshot.HighestUID == 0 {
				t.Fatalf("unexpected inbox snapshot: %+v", snapshot)
			}
			fetched, err := client.FetchUIDRange(ctx, 1, snapshot.HighestUID, 0)
			if err != nil {
				t.Fatalf("imap fetch: %v", err)
			}
			if len(fetched) != 1 {
				t.Fatalf("fetched %d messages, want 1", len(fetched))
			}
			message := fetched[0]
			if message.UID != snapshot.HighestUID {
				t.Fatalf("fetched UID %d, want %d", message.UID, snapshot.HighestUID)
			}
			if !strings.Contains(message.RemoteMessageID, messageID) {
				t.Fatalf("remote message id %q, want it to contain %q", message.RemoteMessageID, messageID)
			}
			parsed, err := ParseMIME(message.Raw)
			if err != nil {
				t.Fatalf("parse fetched message: %v", err)
			}
			if parsed.Subject != subject {
				t.Fatalf("subject %q, want %q", parsed.Subject, subject)
			}
			if len(parsed.From) != 1 || parsed.From[0].Address != "sender@example.invalid" ||
				parsed.From[0].Name != "Synthetic Sender" {
				t.Fatalf("unexpected From: %+v", parsed.From)
			}
			if len(parsed.To) != 2 || parsed.To[0].Address != "user@example.invalid" {
				t.Fatalf("unexpected To: %+v", parsed.To)
			}
			if len(parsed.Cc) != 1 || parsed.Cc[0].Address != "cc-one@example.invalid" {
				t.Fatalf("unexpected Cc: %+v", parsed.Cc)
			}
			if parsed.Date.IsZero() {
				t.Fatal("date header not parsed")
			}
			if strings.TrimSpace(parsed.TextBody) != text {
				t.Fatalf("text body %q, want %q", parsed.TextBody, text)
			}
			// HTML must arrive verbatim (script tag included): sanitization
			// is the upper mailstore/HTTP layer's responsibility.
			if strings.TrimSpace(parsed.HTMLBody) != html {
				t.Fatalf("html body not verbatim: %q", parsed.HTMLBody)
			}
		})
	}
}
