package mailadapter

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	netmail "net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
)

// Submission failure sentinels. Their distinction is the contract required by
// RFC-0003 §4 and plan §3.1 ("结果未知不自动重交"):
//
//   - ErrSubmitRejected: the server explicitly refused, or the transaction
//     failed at a stage where message acceptance is impossible. Definite
//     failure; the upper layer may offer a fresh, user-confirmed retry.
//   - ErrSubmitUnknown: the connection dropped around the final dot, so the
//     server may already have accepted the message. The caller MUST treat
//     this as "result unknown" and MUST NOT resubmit the message
//     automatically (not even with the same Message-ID); resolving the state
//     needs a user action or an out-of-band check.
var (
	ErrSubmitRejected    = errors.New("smtp submission rejected")
	ErrSubmitUnknown     = errors.New("smtp submission result unknown: the server may have accepted the message; never resubmit it automatically")
	ErrSubmissionInvalid = errors.New("smtp submission invalid")
)

// defaultLocalName is the EHLO client name used when the deployment
// configures none. It is a placeholder domain, not a real hostname.
const defaultLocalName = "relay.openalc.invalid"

// SMTPConfig describes one SMTP submission account.
type SMTPConfig struct {
	Host string
	Port int
	TLS  TLSPreference
	// Username and AppPassword authenticate via SASL PLAIN over TLS.
	Username    string
	AppPassword string
	// LocalName is the EHLO client name; empty uses defaultLocalName.
	LocalName string
	// RootCAs optionally overrides the trusted roots (private CA mail
	// hosts). Verification itself is never skipped.
	RootCAs *x509.CertPool
	// Timeout bounds the dial and, as a socket deadline, the whole
	// submission. Zero uses the 60s default.
	Timeout time.Duration
}

func (c SMTPConfig) validate() error {
	if c.Host == "" || c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("%w: smtp endpoint", ErrConfigInvalid)
	}
	if c.TLS != TLSImplicit && c.TLS != TLSStartTLS {
		return fmt.Errorf("%w: smtp TLS mode %q (plaintext is refused by default)", ErrConfigInvalid, string(c.TLS))
	}
	if c.Username == "" || c.AppPassword == "" {
		return ErrCredentialsMissing
	}
	return nil
}

// Submission is one message submission attempt. MessageID must be the stable
// RFC 5322 Message-ID the caller persisted (with the full send intent)
// BEFORE calling Submit; this adapter never generates, rewrites or retries
// one (plan §3.1: SMTP 前先持久化发送意图).
type Submission struct {
	// From is the envelope MAIL FROM address.
	From string
	// To lists the envelope RCPT TO addresses. Bcc recipients belong here
	// but not into Data.
	To []string
	// MessageID is the stable message identifier, with or without angle
	// brackets. It must match the Message-ID header carried in Data.
	MessageID string
	// Data is the full RFC 5322 message with CRLF line endings, headers
	// first, then a blank line, then the body.
	Data []byte
}

func (s Submission) validate() error {
	if _, err := netmail.ParseAddress(s.From); err != nil {
		return fmt.Errorf("%w: MAIL FROM: %v", ErrSubmissionInvalid, err)
	}
	if len(s.To) == 0 {
		return fmt.Errorf("%w: no RCPT TO recipient", ErrSubmissionInvalid)
	}
	for _, to := range s.To {
		if _, err := netmail.ParseAddress(to); err != nil {
			return fmt.Errorf("%w: RCPT TO: %v", ErrSubmissionInvalid, err)
		}
	}
	if normalizeMessageID(s.MessageID) == "" {
		return fmt.Errorf("%w: Message-ID missing", ErrSubmissionInvalid)
	}
	if len(s.Data) == 0 {
		return fmt.Errorf("%w: empty message data", ErrSubmissionInvalid)
	}
	if !dataHasMessageID(s.Data, s.MessageID) {
		return fmt.Errorf("%w: data Message-ID header does not match the submitted identifier", ErrSubmissionInvalid)
	}
	return nil
}

// dataHasMessageID checks that the header block (before the first empty CRLF
// line) carries a Message-ID field matching the submitted identifier.
func dataHasMessageID(data []byte, messageID string) bool {
	headerBlock := data
	if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
		headerBlock = data[:i]
	}
	prefix := "message-id:"
	want := normalizeMessageID(messageID)
	if want == "" {
		return false
	}
	for _, line := range strings.Split(string(headerBlock), "\r\n") {
		if len(line) >= len(prefix) && strings.EqualFold(line[:len(prefix)], prefix) {
			return normalizeMessageID(strings.TrimSpace(line[len(prefix):])) == want
		}
	}
	return false
}

// normalizeMessageID strips surrounding angle brackets and whitespace.
func normalizeMessageID(id string) string {
	return strings.Trim(strings.TrimSpace(id), "<>")
}

// Submit performs one SMTP submission (RFC 6409 semantics: AUTH, MAIL FROM,
// RCPT TO, DATA). It performs exactly one attempt: there is deliberately no
// internal retry and no fallback submission path. If the connection breaks
// after the message data was handed to the server, the returned error wraps
// ErrSubmitUnknown and the caller must record "result unknown".
func Submit(ctx context.Context, cfg SMTPConfig, policy EndpointPolicy, msg Submission) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	if err := policy.Validate(ctx, ServiceSMTP, cfg.Host, cfg.Port); err != nil {
		return err
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	rawConn, err := policy.dialer(ServiceSMTP, cfg.Timeout, ctxDeadline(ctx)).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", cfg.Host, err)
	}
	// One socket deadline bounds the whole submission, including the TLS
	// handshake and every protocol step below.
	if err := rawConn.SetDeadline(sessionDeadline(ctx, cfg.Timeout)); err != nil {
		rawConn.Close()
		return fmt.Errorf("smtp deadline: %w", err)
	}
	client, err := startSMTPSession(ctx, rawConn, cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	localName := cfg.LocalName
	if localName == "" {
		localName = defaultLocalName
	}
	if err := client.Hello(localName); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}
	if err := client.Auth(plainAuthClient{username: cfg.Username, appPassword: cfg.AppPassword}); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(msg.From, nil); err != nil {
		return definiteDataStageError("mail from", err)
	}
	for _, to := range msg.To {
		if err := client.Rcpt(to, nil); err != nil {
			return definiteDataStageError("rcpt to", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		// The DATA transaction never started, so the server cannot have
		// accepted the message: definite failure either way. A protocol
		// refusal is reported as ErrSubmitRejected, an I/O failure as-is.
		return definiteDataStageError("data", err)
	}
	if _, err := writer.Write(msg.Data); err != nil {
		// The terminating dot was never sent, so the server cannot deliver
		// the message: definite failure, not "result unknown".
		_ = writer.Close()
		return fmt.Errorf("smtp submit: %w", err)
	}
	if err := writer.Close(); err != nil {
		// writer.Close sends the final dot and reads the acceptance reply.
		var smtpErr *smtp.SMTPError
		if errors.As(err, &smtpErr) {
			// The server answered the dot with an explicit rejection.
			return fmt.Errorf("%w: %v", ErrSubmitRejected, err)
		}
		// The reply was lost to a connection failure: the server may or may
		// not have accepted the message. This is the "result unknown" case;
		// the caller must never resubmit automatically (RFC-0003 §4).
		return fmt.Errorf("%w: %v", ErrSubmitUnknown, err)
	}
	// The 250 reply for the final dot means the submission was accepted;
	// quitting is best-effort and cannot undo it.
	_ = client.Quit()
	return nil
}

// definiteDataStageError classifies failures that happened before the final
// dot could be sent: the message was not accepted. An explicit server
// refusal is reported through ErrSubmitRejected.
func definiteDataStageError(stage string, err error) error {
	var smtpErr *smtp.SMTPError
	if errors.As(err, &smtpErr) {
		return fmt.Errorf("%w: smtp %s: %v", ErrSubmitRejected, stage, err)
	}
	return fmt.Errorf("smtp %s: %w", stage, err)
}

// plainAuthClient implements the client side of SASL PLAIN (RFC 4616), the
// single mechanism used for app-password authentication over TLS. Keeping it
// here avoids a direct dependency on the go-sasl module; the required
// interface is the two-method shape go-smtp's Client.Auth accepts.
type plainAuthClient struct {
	username    string
	appPassword string
}

func (c plainAuthClient) Start() (string, []byte, error) {
	response := []byte("\x00" + c.username + "\x00" + c.appPassword)
	return "PLAIN", response, nil
}

func (c plainAuthClient) Next(challenge []byte) ([]byte, error) {
	return nil, fmt.Errorf("smtp: unexpected SASL PLAIN challenge (%d bytes)", len(challenge))
}

// startSMTPSession establishes the encrypted transport. Plaintext is
// refused: only implicit TLS and STARTTLS are accepted.
func startSMTPSession(ctx context.Context, rawConn net.Conn, cfg SMTPConfig) (*smtp.Client, error) {
	switch cfg.TLS {
	case TLSImplicit:
		tlsConn := tls.Client(rawConn, tlsConfig(cfg.Host, cfg.RootCAs))
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("smtp tls handshake: %w", err)
		}
		return smtp.NewClient(tlsConn), nil
	case TLSStartTLS:
		client, err := smtp.NewClientStartTLS(rawConn, tlsConfig(cfg.Host, cfg.RootCAs))
		if err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("smtp starttls: %w", err)
		}
		return client, nil
	default:
		rawConn.Close()
		return nil, fmt.Errorf("%w: smtp TLS mode %q", ErrConfigInvalid, string(cfg.TLS))
	}
}
