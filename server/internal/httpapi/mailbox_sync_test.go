package httpapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/mailadapter"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

// The sync endpoints are owner-session mutations over the mail surface. The
// controlled IMAP endpoint is an in-process server on 127.0.0.1 with a
// random port and example.invalid synthetic data; the handler's endpoint
// policy carries the administrator-style allowlist that admits exactly that
// endpoint, and the root store carries its throwaway certificate. Nothing
// here contacts a real mailbox or any external host.

const syncTestIMAPPass = "synthetic-imap-app-password"

type syncTestIMAP struct {
	port int
	user *imapmemserver.User
}

func startSyncTestIMAP(t *testing.T) (syncTestIMAP, *x509.CertPool) {
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
	user := imapmemserver.NewUser("sync@example.invalid", syncTestIMAPPass)
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
	go server.Serve(tls.NewListener(listener, serverTLS))
	var once sync.Once
	t.Cleanup(func() {
		once.Do(func() {
			listener.Close()
			server.Close()
		})
	})
	return syncTestIMAP{port: listener.Addr().(*net.TCPAddr).Port, user: user}, pool
}

func syncTestSeed(t *testing.T, server syncTestIMAP, messageID, subject, body string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("From: Synthetic Sender <sender@example.invalid>\r\n")
	b.WriteString("To: Relay User <user@example.invalid>\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("Date: Fri, 06 Nov 2026 09:30:00 +0000\r\n")
	b.WriteString("Message-ID: <" + messageID + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(body + "\r\n")
	data := []byte(b.String())
	_, err := server.user.Append("INBOX", syncLiteral{
		Reader: bytes.NewReader(data), size: int64(len(data)),
	}, &imap.AppendOptions{Time: time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
}

type syncLiteral struct {
	*bytes.Reader
	size int64
}

func (l syncLiteral) Size() int64 { return l.size }

// syncTestHandler builds the business handler whose endpoint policy admits
// exactly the controlled test endpoint's port. The database and keyring are
// the same shape as the regular business test handler.
func syncTestHandler(t *testing.T) (http.Handler, syncTestIMAP) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := identity.Bootstrap(ctx, db, []byte("local owner passphrase")); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	if err := secrets.CreateFile(keyPath); err != nil {
		t.Fatal(err)
	}
	keys, err := secrets.LoadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	server, rootCAs := startSyncTestIMAP(t)
	policy := mailadapter.EndpointPolicy{
		PrivateAllow: func(_ mailadapter.ServiceKind, _ string, ip net.IP, port int) bool {
			return ip.IsLoopback() && port == server.port
		},
	}
	handler, err := NewBusinessHandlerWithMailSync(db.PingContext, db, keys, "https://relay.test", policy, rootCAs)
	if err != nil {
		t.Fatal(err)
	}
	return handler, server
}

func TestMailSyncRequiresSessionOriginAndCSRF(t *testing.T) {
	handler, _ := syncTestHandler(t)
	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, mailRequest(http.MethodPost, "/v1/mail-accounts/some-id/sync", "", nil, ""))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous sync: %d", anonymous.Code)
	}
	cookie, csrf := loginOwner(t, handler)
	missingOrigin := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://relay.test/v1/mail-accounts/some-id/sync", nil)
	request.Host = "relay.test"
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	handler.ServeHTTP(missingOrigin, request)
	if missingOrigin.Code != http.StatusForbidden {
		t.Fatalf("missing Origin accepted: %d", missingOrigin.Code)
	}
	missingCSRF := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRF, mailRequest(http.MethodPost, "/v1/mail-accounts/some-id/sync", "", cookie, ""))
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF accepted: %d", missingCSRF.Code)
	}
	crossOrigin := httptest.NewRecorder()
	request = mailRequest(http.MethodPost, "/v1/mail-accounts/some-id/sync", "", cookie, csrf)
	request.Header.Set("Origin", "https://evil.example")
	handler.ServeHTTP(crossOrigin, request)
	if crossOrigin.Code != http.StatusForbidden {
		t.Fatalf("cross-origin sync accepted: %d", crossOrigin.Code)
	}
	wrongMethod := httptest.NewRecorder()
	handler.ServeHTTP(wrongMethod, mailRequest(http.MethodGet, "/v1/mail-accounts/some-id/sync", "", cookie, csrf))
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET sync: %d", wrongMethod.Code)
	}
}

func TestMailSyncRoundTripOverHTTP(t *testing.T) {
	handler, server := syncTestHandler(t)
	cookie, csrf := loginOwner(t, handler)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, mailRequest(http.MethodPost, "/v1/mail-accounts", `{"address":"user@example.invalid",
		"imap_host":"127.0.0.1","imap_port":`+strconv.Itoa(server.port)+`,"smtp_host":"smtp.example.invalid",
		"smtp_port":465,"tls_mode":"implicit_tls","username":"sync@example.invalid",
		"password":"`+syncTestIMAPPass+`"}`, cookie, csrf))
	if created.Code != http.StatusCreated {
		t.Fatalf("account create: %d %s", created.Code, created.Body.String())
	}
	var account struct {
		ID string `json:"account_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &account); err != nil || account.ID == "" {
		t.Fatalf("account response: %v", err)
	}
	syncTestSeed(t, server, "batch3-http-1@example.invalid", "HTTP ping one", "Body one.")
	syncTestSeed(t, server, "batch3-http-2@example.invalid", "HTTP ping two", "Body two.")

	result := httptest.NewRecorder()
	handler.ServeHTTP(result, mailRequest(http.MethodPost, "/v1/mail-accounts/"+account.ID+"/sync", "", cookie, csrf))
	if result.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", result.Code, result.Body.String())
	}
	var sync struct {
		UIDValidity      uint32 `json:"uid_validity"`
		FetchedNew       int    `json:"fetched_new"`
		Duplicates       int    `json:"duplicates"`
		Conflicts        int    `json:"conflicts"`
		SkippedMalformed int    `json:"skipped_malformed"`
		HighestUID       uint32 `json:"highest_uid"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &sync); err != nil {
		t.Fatalf("sync response: %v", err)
	}
	if sync.FetchedNew != 2 || sync.UIDValidity == 0 || sync.HighestUID != 2 {
		t.Fatalf("sync result: %+v", sync)
	}
	// The synced records must be visible through the read slice with
	// decrypted subjects and senders.
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, mailRequest(http.MethodGet, "/v1/mail/accounts/"+account.ID+"/messages", "", cookie, csrf))
	if page.Code != http.StatusOK {
		t.Fatalf("list: %d %s", page.Code, page.Body.String())
	}
	var listed struct {
		Messages []struct {
			Subject string `json:"subject"`
			From    string `json:"from"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &listed); err != nil || len(listed.Messages) != 2 {
		t.Fatalf("list body: %v %+v", err, listed)
	}
	// Both seeded messages carry the same internal date, so the keyset
	// order between them is by message id; check content, not order.
	subjects := map[string]bool{}
	for _, message := range listed.Messages {
		if message.From != `"Synthetic Sender" <sender@example.invalid>` {
			t.Fatalf("decrypted sender mismatch: %+v", message)
		}
		subjects[message.Subject] = true
	}
	if !subjects["HTTP ping one"] || !subjects["HTTP ping two"] {
		t.Fatalf("decrypted subjects mismatch: %+v", subjects)
	}

	// The pure connection test reports readiness.
	test := httptest.NewRecorder()
	handler.ServeHTTP(test, mailRequest(http.MethodPost, "/v1/mail-accounts/"+account.ID+"/connection-test", "", cookie, csrf))
	if test.Code != http.StatusOK {
		t.Fatalf("connection test: %d %s", test.Code, test.Body.String())
	}
	var readiness map[string]bool
	if err := json.Unmarshal(test.Body.Bytes(), &readiness); err != nil || !readiness["ok"] || !readiness["imap_ready"] {
		t.Fatalf("connection test body: %s", test.Body.String())
	}
}

func TestMailSyncFailureClassesOverHTTP(t *testing.T) {
	handler, server := syncTestHandler(t)
	cookie, csrf := loginOwner(t, handler)
	create := func(payload string) string {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, mailRequest(http.MethodPost, "/v1/mail-accounts", payload, cookie, csrf))
		if recorder.Code != http.StatusCreated {
			t.Fatalf("account create: %d %s", recorder.Code, recorder.Body.String())
		}
		var account struct {
			ID string `json:"account_id"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &account); err != nil || account.ID == "" {
			t.Fatalf("account response: %v", err)
		}
		return account.ID
	}
	// Rejected credentials classify as an upstream authentication failure.
	bad := create(`{"address":"user@example.invalid","imap_host":"127.0.0.1","imap_port":` + strconv.Itoa(server.port) +
		`,"smtp_host":"smtp.example.invalid","smtp_port":465,"tls_mode":"implicit_tls",
		"username":"sync@example.invalid","password":"deliberately-wrong-password"}`)
	rejected := httptest.NewRecorder()
	handler.ServeHTTP(rejected, mailRequest(http.MethodPost, "/v1/mail-accounts/"+bad+"/sync", "", cookie, csrf))
	if rejected.Code != http.StatusBadGateway || !problemHasCode(rejected, "upstream_auth_failed") {
		t.Fatalf("auth failure: %d %s", rejected.Code, rejected.Body.String())
	}
	authTest := httptest.NewRecorder()
	handler.ServeHTTP(authTest, mailRequest(http.MethodPost, "/v1/mail-accounts/"+bad+"/connection-test", "", cookie, csrf))
	if authTest.Code != http.StatusBadGateway || !problemHasCode(authTest, "upstream_auth_failed") {
		t.Fatalf("connection test auth failure: %d %s", authTest.Code, authTest.Body.String())
	}

	// A private non-loopback host without an allowlist entry is refused
	// before any connection attempt.
	private := create(`{"address":"user@example.invalid","imap_host":"10.0.0.1","imap_port":` + strconv.Itoa(server.port) +
		`,"smtp_host":"smtp.example.invalid","smtp_port":465,"tls_mode":"implicit_tls",
		"username":"sync@example.invalid","password":"` + syncTestIMAPPass + `"}`)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, mailRequest(http.MethodPost, "/v1/mail-accounts/"+private+"/sync", "", cookie, csrf))
	if denied.Code != http.StatusForbidden || !problemHasCode(denied, "endpoint_policy_denied") {
		t.Fatalf("policy denial: %d %s", denied.Code, denied.Body.String())
	}

	// Unknown or other-owner accounts are indistinguishable: 404, and no
	// connection attempt is ever made for them.
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, mailRequest(http.MethodPost, "/v1/mail-accounts/00000000-0000-0000-0000-000000000000/sync", "", cookie, csrf))
	if missing.Code != http.StatusNotFound || !problemHasCode(missing, "not_found") {
		t.Fatalf("unknown account sync: %d %s", missing.Code, missing.Body.String())
	}
	missingTest := httptest.NewRecorder()
	handler.ServeHTTP(missingTest, mailRequest(http.MethodPost, "/v1/mail-accounts/00000000-0000-0000-0000-000000000000/connection-test", "", cookie, csrf))
	if missingTest.Code != http.StatusNotFound {
		t.Fatalf("unknown account connection test: %d", missingTest.Code)
	}
}

// problemHasCode checks the stable code field of a problem+json body.
func problemHasCode(recorder *httptest.ResponseRecorder, code string) bool {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		return false
	}
	return body.Code == code
}

// compile-time guard: the sync test suite needs the same sql handle type the
// handler construction uses.
var _ *sql.DB = (*sql.DB)(nil)
