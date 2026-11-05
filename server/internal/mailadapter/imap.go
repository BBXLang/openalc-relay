package mailadapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// TLSPreference selects the transport encryption mode. The zero value is
// intentionally invalid: plaintext connections are refused by default, only
// implicit TLS and STARTTLS are allowed (RFC 8314).
type TLSPreference string

const (
	TLSImplicit TLSPreference = "implicit_tls"
	TLSStartTLS TLSPreference = "starttls"
)

// Adapter-level sentinels beyond the endpoint policy ones.
var (
	ErrConfigInvalid      = errors.New("mail adapter config invalid")
	ErrCredentialsMissing = errors.New("mail credentials missing")
)

// IMAPConfig describes one IMAP receiving account. The password is the
// user-supplied app password / authorization code; callers must keep it in
// encrypted storage, never in logs.
type IMAPConfig struct {
	Host string
	Port int
	TLS  TLSPreference
	// Username and AppPassword authenticate the LOGIN command.
	Username    string
	AppPassword string
	// RootCAs optionally overrides the trusted roots (private CA mail
	// hosts). Verification itself is never skipped.
	RootCAs *x509.CertPool
	// Timeout bounds the dial and, as a socket deadline, the whole session
	// (login, select, fetches). Zero uses the 60s default.
	Timeout time.Duration
}

func (c IMAPConfig) validate() error {
	if c.Host == "" || c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("%w: imap endpoint", ErrConfigInvalid)
	}
	if c.TLS != TLSImplicit && c.TLS != TLSStartTLS {
		return fmt.Errorf("%w: imap TLS mode %q (plaintext is refused by default)", ErrConfigInvalid, string(c.TLS))
	}
	if c.Username == "" || c.AppPassword == "" {
		return ErrCredentialsMissing
	}
	return nil
}

// InboxSnapshot is the incremental-sync anchor for INBOX. The upper layer
// persists UIDValidity together with HighestUID: if UIDValidity changes the
// mailbox was rebuilt and the sync must restart, otherwise messages with
// UID > HighestUID are new.
type InboxSnapshot struct {
	UIDValidity uint32
	UIDNext     uint32
	// HighestUID is the largest UID the server has assigned so far
	// (UIDNEXT-1), which is the correct watermark even when messages were
	// expunged. Zero for an empty mailbox.
	HighestUID uint32
}

// FetchedMessage is one message fetched from INBOX. Raw is untrusted remote
// input: it must be parsed only through ParseMIME and never executed,
// sanitized here, or trusted in any way.
type FetchedMessage struct {
	UID uint32
	// InternalDate is the server-side IMAP internal date of the message.
	InternalDate time.Time
	// RemoteMessageID is the RFC 5322 Message-ID carried by the IMAP
	// envelope. It is the stable remote identifier the upper layer uses for
	// (account, folder, remote_message_id) deduplication.
	RemoteMessageID string
	Raw             []byte
}

// IMAPClient is a live IMAP session selected on INBOX. Implementations are
// not safe for concurrent use.
type IMAPClient interface {
	// Snapshot returns the INBOX anchor captured at SELECT time.
	Snapshot() InboxSnapshot
	// FetchUIDRange fetches the full raw message plus envelope metadata for
	// every message with UID in [fromUID, toUID]. limit > 0 caps the number
	// of returned messages. The caller is responsible for paging ranges and
	// size limits; this adapter performs no background polling and no
	// automatic reconnection.
	FetchUIDRange(ctx context.Context, fromUID, toUID uint32, limit int) ([]FetchedMessage, error)
	// Close logs out and releases the connection.
	Close() error
}

// Dial connects to the configured IMAP server, authenticates and selects
// INBOX. Connection parameters are validated by the EndpointPolicy before
// and (via the dialer Control hook) during the actual dial.
func Dial(ctx context.Context, cfg IMAPConfig, policy EndpointPolicy) (IMAPClient, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if err := policy.Validate(ctx, ServiceIMAP, cfg.Host, cfg.Port); err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	rawConn, err := policy.dialer(ServiceIMAP, cfg.Timeout, ctxDeadline(ctx)).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("imap dial %s: %w", cfg.Host, err)
	}
	deadline := sessionDeadline(ctx, cfg.Timeout)
	if err := rawConn.SetDeadline(deadline); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("imap deadline: %w", err)
	}
	options := &imapclient.Options{TLSConfig: tlsConfig(cfg.Host, cfg.RootCAs)}
	client, err := dialIMAPSession(ctx, rawConn, cfg.TLS, options)
	if err != nil {
		return nil, err
	}
	if err := client.Login(cfg.Username, cfg.AppPassword).Wait(); err != nil {
		client.Close()
		return nil, fmt.Errorf("imap login: %w", err)
	}
	selectData, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("imap select INBOX: %w", err)
	}
	return &imapSession{
		client: client,
		snapshot: InboxSnapshot{
			UIDValidity: selectData.UIDValidity,
			UIDNext:     uint32(selectData.UIDNext),
			HighestUID:  highestUID(uint32(selectData.UIDNext)),
		},
	}, nil
}

func dialIMAPSession(ctx context.Context, rawConn net.Conn, mode TLSPreference, options *imapclient.Options) (*imapclient.Client, error) {
	switch mode {
	case TLSImplicit:
		tlsConn := tls.Client(rawConn, options.TLSConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("imap tls handshake: %w", err)
		}
		return imapclient.New(tlsConn, options), nil
	case TLSStartTLS:
		client, err := imapclient.NewStartTLS(rawConn, options)
		if err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("imap starttls: %w", err)
		}
		return client, nil
	default:
		rawConn.Close()
		return nil, fmt.Errorf("%w: imap TLS mode %q", ErrConfigInvalid, string(mode))
	}
}

func highestUID(uidNext uint32) uint32 {
	if uidNext <= 1 {
		return 0
	}
	return uidNext - 1
}

// imapSession implements IMAPClient on top of imapclient.Client.
type imapSession struct {
	client   *imapclient.Client
	snapshot InboxSnapshot
}

func (s *imapSession) Snapshot() InboxSnapshot { return s.snapshot }

func (s *imapSession) FetchUIDRange(ctx context.Context, fromUID, toUID uint32, limit int) ([]FetchedMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if toUID < fromUID {
		return nil, nil
	}
	var set imap.UIDSet
	set.AddRange(imap.UID(fromUID), imap.UID(toUID))
	options := &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		InternalDate: true,
		// Zero FetchItemBodySection fetches the whole RFC 5322 message.
		BodySection: []*imap.FetchItemBodySection{{}},
	}
	buffers, err := s.client.Fetch(set, options).Collect()
	if err != nil {
		return nil, fmt.Errorf("imap fetch uid %d:%d: %w", fromUID, toUID, err)
	}
	messages := make([]FetchedMessage, 0, len(buffers))
	for _, buffer := range buffers {
		message := FetchedMessage{
			UID:          uint32(buffer.UID),
			InternalDate: buffer.InternalDate,
			Raw:          append([]byte(nil), buffer.BodySection[0].Bytes...),
		}
		if buffer.Envelope != nil {
			message.RemoteMessageID = buffer.Envelope.MessageID
		}
		messages = append(messages, message)
		if limit > 0 && len(messages) >= limit {
			break
		}
	}
	return messages, nil
}

func (s *imapSession) Close() error {
	logoutErr := s.client.Logout().Wait()
	closeErr := s.client.Close()
	if logoutErr != nil {
		return fmt.Errorf("imap logout: %w", logoutErr)
	}
	return closeErr
}
