package mailsync

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
	"math/big"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/mailadapter"
	"github.com/OpenALC/relay/server/internal/mailstore"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

// Everything in this file runs against an in-process controlled IMAP
// endpoint on 127.0.0.1 with a random port and synthetic example.invalid
// data. Nothing here contacts a real mailbox or any external host.

const (
	testIMAPUser = "sync@example.invalid"
	testIMAPPass = "synthetic-imap-app-password"
)

// loopbackPolicy is the administrator-style allowlist that lets the adapter
// reach exactly the test's controlled 127.0.0.1 endpoint.
func loopbackPolicy(port int) mailadapter.EndpointPolicy {
	return mailadapter.EndpointPolicy{
		PrivateAllow: func(_ mailadapter.ServiceKind, _ string, ip net.IP, p int) bool {
			return ip.IsLoopback() && p == port
		},
	}
}

// testIMAPServer is the controlled endpoint: an in-process IMAP server with
// implicit TLS on 127.0.0.1 and one synthetic user.
type testIMAPServer struct {
	port    int
	rootCAs *x509.CertPool
	user    *imapmemserver.User
	stop    func()
}

func startTestIMAPServer(t *testing.T) testIMAPServer {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
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
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}},
		MinVersion:   tls.VersionTLS12,
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	memory := imapmemserver.New()
	user := imapmemserver.NewUser(testIMAPUser, testIMAPPass)
	memory.AddUser(user)
	if err := user.Create("INBOX", nil); err != nil {
		t.Fatal(err)
	}
	server := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memory.NewSession(), nil, nil
		},
		TLSConfig: serverTLS,
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsListener := tls.NewListener(listener, serverTLS)
	go server.Serve(tlsListener)
	var once sync.Once
	stop := func() {
		once.Do(func() {
			listener.Close()
			server.Close()
		})
	}
	t.Cleanup(stop)
	return testIMAPServer{
		port:    listener.Addr().(*net.TCPAddr).Port,
		rootCAs: pool,
		user:    user,
		stop:    stop,
	}
}

// seedMessage is a deterministic single-part RFC 5322 message.
func seedMessage(messageID, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: Synthetic Sender <sender@example.invalid>\r\n")
	b.WriteString("To: Relay User <user@example.invalid>\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("Date: Fri, 06 Nov 2026 09:30:00 +0000\r\n")
	b.WriteString("Message-ID: <" + messageID + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body + "\r\n")
	return []byte(b.String())
}

func (s testIMAPServer) append(t *testing.T, data []byte) {
	t.Helper()
	_, err := s.user.Append("INBOX", literalReader{
		Reader: bytes.NewReader(data), size: int64(len(data)),
	}, &imap.AppendOptions{Time: time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("seed imap message: %v", err)
	}
}

// literalReader adapts a byte slice to the IMAP literal reader.
type literalReader struct {
	*bytes.Reader
	size int64
}

func (l literalReader) Size() int64 { return l.size }

// fixture wires the encrypted store, one owner and the controlled endpoint.
type fixture struct {
	syncer  Syncer
	store   mailstore.Store
	ownerID string
	server  testIMAPServer
	policy  mailadapter.EndpointPolicy
	now     time.Time
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	keys, err := secrets.LoadFile(func() string {
		path := filepath.Join(t.TempDir(), "keys.json")
		if err := secrets.CreateFile(path); err != nil {
			t.Fatal(err)
		}
		return path
	}())
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := identity.Bootstrap(ctx, db, []byte("synthetic owner password")); err != nil {
		t.Fatal(err)
	}
	ownerID, ok, err := identity.VerifyPassword(ctx, db, []byte("synthetic owner password"))
	if err != nil || !ok {
		t.Fatalf("read owner: %v", err)
	}
	server := startTestIMAPServer(t)
	store := mailstore.Store{DB: db, Keys: keys}
	return fixture{
		syncer:  Syncer{Store: &store, Keys: keys, RootCAs: server.rootCAs},
		store:   store,
		ownerID: ownerID,
		server:  server,
		policy:  loopbackPolicy(server.port),
		now:     time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
	}
}

// account registers one account pointed at the controlled endpoint. The
// password may deliberately mismatch the server user for auth-failure tests.
func (f fixture) account(t *testing.T, password string) mailstore.Account {
	t.Helper()
	account, err := f.store.CreateAccount(context.Background(), f.ownerID, mailstore.CreateAccountInput{
		Address:  "user@example.invalid",
		IMAPHost: "127.0.0.1",
		IMAPPort: f.server.port,
		SMTPHost: "smtp.example.invalid",
		SMTPPort: 465,
		TLSMode:  mailstore.TLSImplicitTLS,
		Username: testIMAPUser,
		Password: password,
	}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func TestSyncInboxImportsAndDecrypts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t, testIMAPPass)
	for i := 1; i <= 5; i++ {
		f.server.append(t, seedMessage(
			"batch3-sync-"+string(rune('0'+i))+"@example.invalid",
			"Synthetic ping "+string(rune('0'+i)),
			"Body number "+string(rune('0'+i))+".",
		))
	}
	result, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.FetchedNew != 5 || result.Duplicates != 0 || result.Conflicts != 0 ||
		result.SkippedMalformed != 0 || result.UIDValidity == 0 || result.HighestUID != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
	page, err := f.store.ListMessages(ctx, f.ownerID, mailstore.ListOptions{AccountID: account.ID, Limit: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Messages) != 5 || page.NextCursor != "" {
		t.Fatalf("stored page: %d messages, next_cursor %q", len(page.Messages), page.NextCursor)
	}
	// The decrypted fields must round-trip through the sealed storage.
	seen := map[string]bool{}
	for _, summary := range page.Messages {
		detail, err := f.store.GetMessage(ctx, f.ownerID, summary.ID)
		if err != nil {
			t.Fatalf("get message: %v", err)
		}
		if detail.Folder != FolderInbox || detail.RemoteMessageID == "" {
			t.Fatalf("unexpected detail: %+v", detail)
		}
		if !strings.HasPrefix(detail.Subject, "Synthetic ping ") ||
			!strings.HasPrefix(detail.Body, "Body number ") {
			t.Fatalf("decrypted content mismatch: %+v", detail)
		}
		if detail.Headers.From != `"Synthetic Sender" <sender@example.invalid>` {
			t.Fatalf("from mismatch: %q", detail.Headers.From)
		}
		if len(detail.Headers.To) != 1 || detail.Headers.To[0] != `"Relay User" <user@example.invalid>` {
			t.Fatalf("to mismatch: %+v", detail.Headers.To)
		}
		seen[detail.Subject] = true
	}
	for i := 1; i <= 5; i++ {
		if !seen["Synthetic ping "+string(rune('0'+i))] {
			t.Fatalf("message %d missing from store", i)
		}
	}
	_, credential, anchor, err := f.store.OpenSyncState(ctx, f.ownerID, account.ID)
	if err != nil {
		t.Fatalf("open sync state: %v", err)
	}
	if credential != testIMAPPass {
		t.Fatal("credential did not round-trip")
	}
	if !anchor.Set || anchor.UIDValidity != result.UIDValidity || anchor.HighestUID != 5 {
		t.Fatalf("anchor not persisted: %+v", anchor)
	}
	updated, err := f.store.GetAccount(ctx, f.ownerID, account.ID)
	if err != nil || updated.ConnectionState != mailstore.ConnectionOK || updated.LastSyncAt == nil {
		t.Fatalf("account state not updated: %+v, %v", updated, err)
	}
}

func TestSyncInboxIncremental(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t, testIMAPPass)
	for i := 0; i < 5; i++ {
		f.server.append(t, seedMessage(messageID(i), "Synthetic ping", "Body."))
	}
	first, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil || first.FetchedNew != 5 {
		t.Fatalf("first sync: %+v, %v", first, err)
	}
	// No new mail: the fetch window above the watermark is empty.
	second, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if second.FetchedNew != 0 || second.Duplicates != 0 || second.Conflicts != 0 ||
		second.HighestUID != first.HighestUID {
		t.Fatalf("no-op sync counted work: %+v", second)
	}
	// Two new messages: only the range above the watermark is fetched.
	f.server.append(t, seedMessage(messageID(5), "Synthetic ping", "Body."))
	f.server.append(t, seedMessage(messageID(6), "Synthetic ping", "Body."))
	third, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil {
		t.Fatalf("third sync: %v", err)
	}
	if third.FetchedNew != 2 || third.Duplicates != 0 || third.HighestUID != 7 {
		t.Fatalf("incremental sync: %+v", third)
	}
	page, err := f.store.ListMessages(ctx, f.ownerID, mailstore.ListOptions{AccountID: account.ID, Limit: 100})
	if err != nil || len(page.Messages) != 7 {
		t.Fatalf("store count: %d, %v", len(page.Messages), err)
	}
}

func messageID(i int) string {
	return "batch3-sync-" + string(rune('0'+i)) + "@example.invalid"
}

func TestSyncInboxUIDValidityRestart(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t, testIMAPPass)
	for i := 0; i < 5; i++ {
		f.server.append(t, seedMessage(messageID(i), "Synthetic ping", "Body."))
	}
	first, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil || first.FetchedNew != 5 {
		t.Fatalf("first sync: %+v, %v", first, err)
	}
	// Rebuild the mailbox: UIDVALIDITY must change, the sync restarts the
	// pull from UID 1 and the store deduplicates every message.
	if err := f.server.user.Delete("INBOX"); err != nil {
		t.Fatal(err)
	}
	if err := f.server.user.Create("INBOX", nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		f.server.append(t, seedMessage(messageID(i), "Synthetic ping", "Body."))
	}
	second, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil {
		t.Fatalf("post-rebuild sync: %v", err)
	}
	if second.UIDValidity == first.UIDValidity {
		t.Fatalf("uidvalidity did not change: %d", second.UIDValidity)
	}
	if second.FetchedNew != 0 || second.Duplicates != 5 || second.Conflicts != 0 {
		t.Fatalf("rebuild sync: %+v", second)
	}
	page, err := f.store.ListMessages(ctx, f.ownerID, mailstore.ListOptions{AccountID: account.ID, Limit: 100})
	if err != nil || len(page.Messages) != 5 {
		t.Fatalf("store count after rebuild: %d, %v", len(page.Messages), err)
	}
}

func TestSyncInboxConflictKeepsStoredRecord(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t, testIMAPPass)
	f.server.append(t, seedMessage("batch3-conflict@example.invalid", "Original subject", "Original body."))
	first, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil || first.FetchedNew != 1 {
		t.Fatalf("first sync: %+v, %v", first, err)
	}
	stored, err := f.store.ListMessages(ctx, f.ownerID, mailstore.ListOptions{AccountID: account.ID})
	if err != nil || len(stored.Messages) != 1 {
		t.Fatalf("stored page: %+v, %v", stored, err)
	}
	original, err := f.store.GetMessage(ctx, f.ownerID, stored.Messages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	// Same Message-ID, different content: a conflict, never an overwrite.
	f.server.append(t, seedMessage("batch3-conflict@example.invalid", "Forged subject", "Forged body."))
	second, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil {
		t.Fatalf("conflict sync: %v", err)
	}
	if second.Conflicts != 1 || second.FetchedNew != 0 || second.Duplicates != 0 {
		t.Fatalf("conflict sync result: %+v", second)
	}
	after, err := f.store.GetMessage(ctx, f.ownerID, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Subject != "Original subject" || strings.TrimRight(after.Body, "\r\n") != "Original body." {
		t.Fatalf("stored record was overwritten: %+v", after)
	}
}

func TestSyncInboxMalformedSkipped(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t, testIMAPPass)
	f.server.append(t, seedMessage("batch3-good@example.invalid", "Good subject", "Good body."))
	f.server.append(t, []byte("this is not MIME at all\x00\x01"))
	f.server.append(t, seedMessage("batch3-good-2@example.invalid", "Second good", "Second body."))
	result, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil {
		t.Fatalf("sync with malformed message: %v", err)
	}
	if result.FetchedNew != 2 || result.SkippedMalformed != 1 {
		t.Fatalf("malformed handling: %+v", result)
	}
	// The watermark still advanced past the skipped UID: it is not
	// re-fetched forever.
	if result.HighestUID != 3 {
		t.Fatalf("watermark did not advance past skipped message: %+v", result)
	}
	page, err := f.store.ListMessages(ctx, f.ownerID, mailstore.ListOptions{AccountID: account.ID})
	if err != nil || len(page.Messages) != 2 {
		t.Fatalf("store count: %d, %v", len(page.Messages), err)
	}
}

func TestSyncInboxOversizedBodySkipped(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t, testIMAPPass)
	f.server.append(t, seedMessage("batch3-big@example.invalid", "Big subject", strings.Repeat("B", 2<<20)))
	f.server.append(t, seedMessage("batch3-small@example.invalid", "Small subject", "Small body."))
	result, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil {
		t.Fatalf("sync with oversized body: %v", err)
	}
	if result.FetchedNew != 1 || result.SkippedMalformed != 1 {
		t.Fatalf("oversized handling: %+v", result)
	}
	// The skipped record must not deadlock the next round either.
	again, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy)
	if err != nil || again.FetchedNew != 0 || again.SkippedMalformed != 0 {
		t.Fatalf("follow-up sync: %+v, %v", again, err)
	}
}

func TestSyncInboxAuthRejected(t *testing.T) {
	f := newFixture(t)
	account := f.account(t, "deliberately-wrong-password")
	_, err := f.syncer.SyncInbox(context.Background(), f.ownerID, account.ID, f.policy)
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("auth failure misclassified: %v", err)
	}
}

func TestSyncInboxPolicyDeniedBeforeDial(t *testing.T) {
	f := newFixture(t)
	account := f.account(t, testIMAPPass)
	// The strict production policy has no allowlist hook: the loopback
	// endpoint must be refused before any connection is attempted.
	_, err := f.syncer.SyncInbox(context.Background(), f.ownerID, account.ID, mailadapter.EndpointPolicy{})
	if !errors.Is(err, ErrPolicy) {
		t.Fatalf("policy denial misclassified: %v", err)
	}
}

func TestSyncInboxConnectRefused(t *testing.T) {
	f := newFixture(t)
	account := f.account(t, testIMAPPass)
	// Stop the controlled endpoint: it now refuses connections. The
	// allowlist still admits the port, so the failure classifies as connect.
	f.server.stop()
	_, err := f.syncer.SyncInbox(context.Background(), f.ownerID, account.ID, f.policy)
	if !errors.Is(err, ErrConnect) {
		t.Fatalf("connect failure misclassified: %v", err)
	}
}

func TestSyncInboxScopingAndRevocation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t, testIMAPPass)
	if _, err := f.syncer.SyncInbox(ctx, "other-owner", account.ID, f.policy); !errors.Is(err, mailstore.ErrNotFound) {
		t.Fatalf("cross-owner sync: %v", err)
	}
	if err := f.store.RevokeAccount(ctx, f.ownerID, account.ID, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.syncer.SyncInbox(ctx, f.ownerID, account.ID, f.policy); !errors.Is(err, mailstore.ErrDenied) {
		t.Fatalf("revoked account sync: %v", err)
	}
}

func TestConnectionTestStates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	good := f.account(t, testIMAPPass)
	if err := f.syncer.TestConnection(ctx, f.ownerID, good.ID, f.policy); err != nil {
		t.Fatalf("connection test with valid credentials: %v", err)
	}
	// The pure test must not touch any stored state.
	account, err := f.store.GetAccount(ctx, f.ownerID, good.ID)
	if err != nil || account.ConnectionState != mailstore.ConnectionUntested || account.LastSyncAt != nil {
		t.Fatalf("connection test mutated storage: %+v, %v", account, err)
	}
	bad := f.account(t, "deliberately-wrong-password")
	if err := f.syncer.TestConnection(ctx, f.ownerID, bad.ID, f.policy); !errors.Is(err, ErrAuth) {
		t.Fatalf("connection test with rejected credentials: %v", err)
	}
	if err := f.syncer.TestConnection(ctx, "other-owner", good.ID, f.policy); !errors.Is(err, mailstore.ErrNotFound) {
		t.Fatalf("cross-owner connection test: %v", err)
	}
}
