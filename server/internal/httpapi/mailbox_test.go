package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/mailstore"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/smsstore"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
	"github.com/OpenALC/relay/server/internal/synthetic"
)

// The read slices expose the batch-2 encrypted stores over HTTP. Synthetic
// records use example.invalid addresses and fixed bodies only. The schema
// keeps a single owner per database (owners.singleton CHECK plus foreign
// keys), so cross-owner isolation is proven with two independent fixtures:
// each holds its own owner, database and keyring, mirroring owner A and
// owner B deployments.

type readFixture struct {
	handler  http.Handler
	db       *sql.DB
	mail     mailstore.Store
	sms      smsstore.Store
	pairings synthetic.Store
	ownerID  string
	baseTime time.Time
}

func newReadFixture(t *testing.T) readFixture {
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
	ownerID, ok, err := identity.VerifyPassword(ctx, db, []byte("local owner passphrase"))
	if err != nil || !ok {
		t.Fatalf("read owner: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	if err := secrets.CreateFile(keyPath); err != nil {
		t.Fatal(err)
	}
	keys, err := secrets.LoadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewBusinessHandler(db.PingContext, db, keys, "https://relay.test")
	if err != nil {
		t.Fatal(err)
	}
	return readFixture{
		handler: handler, db: db,
		mail:     mailstore.Store{DB: db, Keys: keys},
		sms:      smsstore.Store{DB: db, Keys: keys},
		pairings: synthetic.Store{DB: db},
		ownerID:  ownerID,
		baseTime: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
	}
}

func (f readFixture) mailAccount(t *testing.T) mailstore.Account {
	t.Helper()
	account, err := f.mail.CreateAccount(context.Background(), f.ownerID, mailstore.CreateAccountInput{
		Address: "user@example.invalid", IMAPHost: "imap.example.invalid", IMAPPort: 993,
		SMTPHost: "smtp.example.invalid", SMTPPort: 465, TLSMode: mailstore.TLSImplicitTLS,
		Username: "user@example.invalid", Password: "synthetic-app-password",
	}, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func (f readFixture) mailInput(accountID, remoteID, subject, body string, receivedAt time.Time) mailstore.MessageInput {
	return mailstore.MessageInput{
		AccountID: accountID, Folder: "INBOX", RemoteMessageID: remoteID,
		ReceivedAt: receivedAt, SizeBytes: int64(len(body)), Unread: true,
		HasAttachments: false, BodyFormat: mailstore.BodyTextPlain, Subject: subject,
		Headers: mailstore.MessageHeaders{From: "sender@example.invalid", To: []string{"user@example.invalid"}},
		Body:    body,
	}
}

// device pairs a synthetic device, mirroring the owner approved flow.
func (f readFixture) device(t *testing.T, label string) synthetic.Device {
	t.Helper()
	ctx := context.Background()
	pair, err := f.pairings.CreatePairing(ctx, f.ownerID, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	request, err := f.pairings.RequestPairing(ctx, pair.ID, label, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.pairings.ApprovePairing(ctx, f.ownerID, pair.ID, request.ID, request.ComparisonCode, f.baseTime); err != nil {
		t.Fatal(err)
	}
	device, err := f.pairings.ExchangePairing(ctx, pair.ID, request.ID, request.ClaimToken, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	return device
}

func (f readFixture) smsInput(deviceID, remoteID, body string, deviceAt time.Time) smsstore.MessageInput {
	return smsstore.MessageInput{
		DeviceID: deviceID, Direction: smsstore.DirectionInbound, RemoteEventID: remoteID,
		Sender: "10086", Recipient: "1069000000000", Body: body, DeviceAt: deviceAt,
	}
}

func (f readFixture) mailID(t *testing.T, remoteID string) string {
	t.Helper()
	var id string
	if err := f.db.QueryRow(`SELECT message_id FROM mail_messages
		WHERE remote_message_id = ?`, remoteID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f readFixture) smsID(t *testing.T, remoteID string) string {
	t.Helper()
	var id string
	if err := f.db.QueryRow(`SELECT sms_id FROM sms_messages
		WHERE remote_event_id = ?`, remoteID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// ownerSession logs in exactly once per fixture: the login rate limit is
// global per handler and the read endpoints share one session cookie.
func ownerSession(t *testing.T, f readFixture) (*http.Cookie, string) {
	t.Helper()
	return loginOwner(t, f.handler)
}

func readGet(t *testing.T, handler http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, mailRequest(http.MethodGet, path, "", cookie, ""))
	return recorder
}

func wantProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status: %d %s", recorder.Code, recorder.Body.String())
	}
	if media := recorder.Header().Get("Content-Type"); media != "application/problem+json" {
		t.Fatalf("problem content type: %s", media)
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil || problem.Code != code {
		t.Fatalf("problem body: %s", recorder.Body.String())
	}
}

// decodeBody always receives a fresh target: json.Unmarshal merges into the
// destination and would otherwise keep fields that the next response omitted.
func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("response body: %v: %s", err, recorder.Body.String())
	}
}

type mailSummary struct {
	ID      string `json:"message_id"`
	Subject string `json:"subject"`
	From    string `json:"from"`
	Preview string `json:"preview"`
}

type mailListPage struct {
	Messages   []mailSummary `json:"messages"`
	NextCursor string        `json:"next_cursor"`
}

type mailDetailBody struct {
	MessageID  string   `json:"message_id"`
	Subject    string   `json:"subject"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	BodyFormat string   `json:"body_format"`
	BodyText   string   `json:"body_text"`
	BodyHTML   string   `json:"body_html"`
	HTMLNotice string   `json:"html_notice"`
}

type smsSummary struct {
	ID        string `json:"sms_id"`
	DeviceID  string `json:"device_id"`
	Direction string `json:"direction"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
}

type smsListPage struct {
	Messages   []smsSummary `json:"messages"`
	NextCursor string       `json:"next_cursor"`
}

func TestMailReadEndpointsRequireOwnerSession(t *testing.T) {
	f := newReadFixture(t)
	account := f.mailAccount(t)
	unknownID := "00000000-0000-4000-8000-000000000000"
	paths := []string{
		"/v1/mail/accounts/" + account.ID + "/messages",
		"/v1/mail/messages/" + unknownID,
		"/v1/mail/search?q=synthetic",
		"/v1/sms/messages",
		"/v1/sms/messages/" + unknownID,
		"/v1/sms/search?q=synthetic",
	}
	for _, path := range paths {
		wantProblem(t, readGet(t, f.handler, path, nil), http.StatusUnauthorized, "session_required")
	}
	cookie, csrf := ownerSession(t, f)
	for _, path := range paths {
		// Pure GET checks the session only: no Origin and no CSRF header.
		if authorized := readGet(t, f.handler, path, cookie); authorized.Code == http.StatusUnauthorized ||
			authorized.Code == http.StatusForbidden {
			t.Fatalf("GET %s rejected without CSRF: %d %s", path, authorized.Code, authorized.Body.String())
		}
	}
	// Origin rules protect mutations, not reads: a cross-site read with a valid
	// session still succeeds.
	request := mailRequest(http.MethodGet, "/v1/sms/messages", "", cookie, "")
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	crossOrigin := httptest.NewRecorder()
	f.handler.ServeHTTP(crossOrigin, request)
	if crossOrigin.Code != http.StatusOK {
		t.Fatalf("cross-origin read rejected: %d %s", crossOrigin.Code, crossOrigin.Body.String())
	}
	// Writes do not exist on the read surface.
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v1/sms/messages"},
		{http.MethodPost, "/v1/mail/search"},
		{http.MethodDelete, "/v1/mail/messages/" + unknownID},
		{http.MethodPost, "/v1/mail/accounts/" + account.ID + "/messages"},
	} {
		recorder := httptest.NewRecorder()
		f.handler.ServeHTTP(recorder, mailRequest(route.method, route.path, "", cookie, csrf))
		wantProblem(t, recorder, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func TestMailInboxListPaginationAndIsolation(t *testing.T) {
	f := newReadFixture(t)
	other := newReadFixture(t)
	ctx := context.Background()
	cookie, _ := ownerSession(t, f)
	account := f.mailAccount(t)
	ids := make([]string, 0, 5)
	for i := range 5 {
		receivedAt := f.baseTime.Add(time.Duration(i) * time.Minute)
		body := "synthetic body " + strings.Repeat("filler ", 25) + "tailsecret"
		input := f.mailInput(account.ID, "<list-"+string(rune('a'+i))+"@example.invalid>",
			"synthetic subject "+string(rune('a'+i)), body, receivedAt)
		if _, err := f.mail.InsertMessage(ctx, f.ownerID, input, receivedAt); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, f.mailID(t, input.RemoteMessageID))
	}
	listPath := "/v1/mail/accounts/" + account.ID + "/messages"
	full := readGet(t, f.handler, listPath, cookie)
	if full.Code != http.StatusOK {
		t.Fatalf("list: %d %s", full.Code, full.Body.String())
	}
	var page mailListPage
	decodeBody(t, full, &page)
	if len(page.Messages) != 5 || page.Messages[0].ID != ids[4] || page.NextCursor != "" {
		t.Fatalf("full page order: %+v cursor %q", page.Messages, page.NextCursor)
	}
	raw := full.Body.String()
	if strings.Contains(raw, `"body"`) || strings.Contains(raw, "tailsecret") {
		t.Fatal("list exposes the full body")
	}
	if !strings.Contains(raw, `"preview"`) || page.Messages[0].Preview == "" {
		t.Fatal("list preview missing")
	}
	// Stable pagination: records arriving after the cursor never shift it.
	first := readGet(t, f.handler, listPath+"?limit=2", cookie)
	page = mailListPage{}
	decodeBody(t, first, &page)
	if len(page.Messages) != 2 || page.Messages[0].ID != ids[4] || page.Messages[1].ID != ids[3] || page.NextCursor == "" {
		t.Fatalf("first page: %+v %s", page.Messages, first.Body.String())
	}
	later := f.mailInput(account.ID, "<list-new@example.invalid>", "later subject",
		"synthetic later body", f.baseTime.Add(10*time.Minute))
	if _, err := f.mail.InsertMessage(ctx, f.ownerID, later, f.baseTime.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	second := readGet(t, f.handler, listPath+"?limit=2&cursor="+page.NextCursor, cookie)
	page = mailListPage{}
	decodeBody(t, second, &page)
	if len(page.Messages) != 2 || page.Messages[0].ID != ids[2] || page.Messages[1].ID != ids[1] {
		t.Fatalf("second page shifted: %+v", page.Messages)
	}
	third := readGet(t, f.handler, listPath+"?limit=2&cursor="+page.NextCursor, cookie)
	page = mailListPage{}
	decodeBody(t, third, &page)
	if len(page.Messages) != 1 || page.Messages[0].ID != ids[0] || page.NextCursor != "" {
		t.Fatalf("third page: %+v cursor %q", page.Messages, page.NextCursor)
	}
	for _, invalid := range []string{"?cursor=broken", "?limit=0", "?limit=101", "?limit=abc", "?cursor=&bogus=1"} {
		wantProblem(t, readGet(t, f.handler, listPath+invalid, cookie), http.StatusBadRequest, "invalid_request")
	}
	single := readGet(t, f.handler, listPath+"?limit=1", cookie)
	page = mailListPage{}
	decodeBody(t, single, &page)
	if len(page.Messages) != 1 || page.Messages[0].Subject != "later subject" {
		t.Fatalf("limit=1 page: %+v", page.Messages)
	}
	// Owner B's data lives on its own fixture: owner A's session gets 404 for
	// the account and never sees B's content in a search.
	otherAccount := other.mailAccount(t)
	otherInput := other.mailInput(otherAccount.ID, "<other-owner@example.invalid>", "other owner subject",
		"other owner synthetic body", other.baseTime)
	if _, err := other.mail.InsertMessage(ctx, other.ownerID, otherInput, other.baseTime); err != nil {
		t.Fatal(err)
	}
	wantProblem(t, readGet(t, f.handler, "/v1/mail/accounts/"+otherAccount.ID+"/messages", cookie),
		http.StatusNotFound, "not_found")
	wantProblem(t, readGet(t, f.handler, "/v1/mail/accounts/00000000-0000-4000-8000-000000000000/messages", cookie),
		http.StatusNotFound, "not_found")
	misses := readGet(t, f.handler, "/v1/mail/search?q=other+owner+synthetic", cookie)
	page = mailListPage{}
	decodeBody(t, misses, &page)
	if len(page.Messages) != 0 {
		t.Fatalf("search crossed the owner scope: %+v", page.Messages)
	}
}

func TestMailMessageDetailBodyHandling(t *testing.T) {
	f := newReadFixture(t)
	other := newReadFixture(t)
	ctx := context.Background()
	cookie, _ := ownerSession(t, f)
	account := f.mailAccount(t)
	textInput := f.mailInput(account.ID, "<plain@example.invalid>", "plain subject",
		"synthetic plain body", f.baseTime)
	if _, err := f.mail.InsertMessage(ctx, f.ownerID, textInput, f.baseTime); err != nil {
		t.Fatal(err)
	}
	htmlInput := f.mailInput(account.ID, "<html@example.invalid>", "html subject",
		"<p>synthetic html body</p>", f.baseTime.Add(time.Minute))
	htmlInput.BodyFormat = mailstore.BodySanitizedHTML
	if _, err := f.mail.InsertMessage(ctx, f.ownerID, htmlInput, f.baseTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	textID := f.mailID(t, textInput.RemoteMessageID)
	htmlID := f.mailID(t, htmlInput.RemoteMessageID)

	plain := readGet(t, f.handler, "/v1/mail/messages/"+textID, cookie)
	if plain.Code != http.StatusOK {
		t.Fatalf("text detail: %d %s", plain.Code, plain.Body.String())
	}
	var detail mailDetailBody
	decodeBody(t, plain, &detail)
	if detail.BodyText != "synthetic plain body" || detail.BodyHTML != "" || detail.HTMLNotice != "" ||
		detail.Subject != "plain subject" || detail.From != "sender@example.invalid" ||
		len(detail.To) != 1 || detail.To[0] != "user@example.invalid" {
		t.Fatalf("text detail fields: %+v", detail)
	}
	if strings.Contains(plain.Body.String(), "body_html") || strings.Contains(plain.Body.String(), "html_notice") {
		t.Fatal("text detail advertises html fields")
	}
	// Without the opt-in the stored HTML body is withheld and explained.
	html := readGet(t, f.handler, "/v1/mail/messages/"+htmlID, cookie)
	detail = mailDetailBody{}
	decodeBody(t, html, &detail)
	if detail.BodyHTML != "" || detail.BodyText != "" || detail.HTMLNotice == "" ||
		!strings.Contains(detail.HTMLNotice, "not returned by default") {
		t.Fatalf("html detail without opt-in: %+v", detail)
	}
	optIn := readGet(t, f.handler, "/v1/mail/messages/"+htmlID+"?format=html", cookie)
	detail = mailDetailBody{}
	decodeBody(t, optIn, &detail)
	if detail.BodyHTML != "<p>synthetic html body</p>" || detail.BodyText != "" ||
		!strings.Contains(detail.HTMLNotice, "no additional sanitizer") {
		t.Fatalf("html detail with opt-in: %+v", detail)
	}
	textOptIn := readGet(t, f.handler, "/v1/mail/messages/"+textID+"?format=html", cookie)
	detail = mailDetailBody{}
	decodeBody(t, textOptIn, &detail)
	if detail.BodyText != "synthetic plain body" || detail.BodyHTML != "" {
		t.Fatalf("text preferred over html opt-in: %+v", detail)
	}
	wantProblem(t, readGet(t, f.handler, "/v1/mail/messages/"+textID+"?format=markdown", cookie),
		http.StatusBadRequest, "invalid_request")
	wantProblem(t, readGet(t, f.handler, "/v1/mail/messages/00000000-0000-4000-8000-000000000000", cookie),
		http.StatusNotFound, "not_found")
	// Owner B's message is 404 for owner A, never a partial or foreign view.
	otherAccount := other.mailAccount(t)
	otherInput := other.mailInput(otherAccount.ID, "<other-detail@example.invalid>", "other owner subject",
		"other owner synthetic body", other.baseTime)
	if _, err := other.mail.InsertMessage(ctx, other.ownerID, otherInput, other.baseTime); err != nil {
		t.Fatal(err)
	}
	wantProblem(t, readGet(t, f.handler, "/v1/mail/messages/"+other.mailID(t, otherInput.RemoteMessageID), cookie),
		http.StatusNotFound, "not_found")
}

func TestMailSearchScopingPaginationAndQueryRequired(t *testing.T) {
	f := newReadFixture(t)
	other := newReadFixture(t)
	ctx := context.Background()
	cookie, _ := ownerSession(t, f)
	account := f.mailAccount(t)
	second := f.mailAccount(t) // second account of the same owner
	insert := func(target mailstore.Account, remoteID, subject, body string, at time.Time) {
		t.Helper()
		input := f.mailInput(target.ID, remoteID, subject, body, at)
		if _, err := f.mail.InsertMessage(ctx, f.ownerID, input, at); err != nil {
			t.Fatal(err)
		}
	}
	insert(account, "<s-target@example.invalid>", "quarterly report",
		"the deployment window opens next week", f.baseTime)
	insert(second, "<s-other@example.invalid>", "deployment plan",
		"lunch menu soup today", f.baseTime.Add(time.Minute))
	// Owner B stores a copy of the same content on its own fixture.
	otherAccount := other.mailAccount(t)
	otherInput := other.mailInput(otherAccount.ID, "<s-foreign@example.invalid>", "deployment",
		"deployment window", other.baseTime.Add(2*time.Minute))
	if _, err := other.mail.InsertMessage(ctx, other.ownerID, otherInput, other.baseTime.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}

	for _, missing := range []string{"", "?q=", "?q=%20%20%20", "?account_id=" + account.ID} {
		wantProblem(t, readGet(t, f.handler, "/v1/mail/search"+missing, cookie),
			http.StatusBadRequest, "invalid_request")
	}
	var page mailListPage
	hits := readGet(t, f.handler, "/v1/mail/search?q=deployment+window", cookie)
	if hits.Code != http.StatusOK {
		t.Fatalf("search: %d %s", hits.Code, hits.Body.String())
	}
	decodeBody(t, hits, &page)
	if len(page.Messages) != 1 || page.Messages[0].Subject != "quarterly report" || page.NextCursor != "" {
		t.Fatalf("body hit: %+v %s", page.Messages, hits.Body.String())
	}
	caseInsensitive := readGet(t, f.handler, "/v1/mail/search?q=QUARTERLY", cookie)
	page = mailListPage{}
	decodeBody(t, caseInsensitive, &page)
	if len(page.Messages) != 1 {
		t.Fatalf("subject hit: %+v", page.Messages)
	}
	senderHit := readGet(t, f.handler, "/v1/mail/search?q=sender%40example.invalid", cookie)
	page = mailListPage{}
	decodeBody(t, senderHit, &page)
	// Both owner messages (target and the second account's) share the sender.
	if len(page.Messages) != 2 {
		t.Fatalf("sender hit: %d messages", len(page.Messages))
	}
	scoped := readGet(t, f.handler, "/v1/mail/search?q=deployment&account_id="+second.ID, cookie)
	page = mailListPage{}
	decodeBody(t, scoped, &page)
	if len(page.Messages) != 1 || page.Messages[0].Subject != "deployment plan" {
		t.Fatalf("account scope: %+v", page.Messages)
	}
	// Owner B's copy (subject "deployment") is invisible to owner A even under
	// an exact query: only owner A's own "quarterly report" copy can appear.
	unscoped := readGet(t, f.handler, "/v1/mail/search?q=deployment+window", cookie)
	page = mailListPage{}
	decodeBody(t, unscoped, &page)
	if len(page.Messages) != 1 || page.Messages[0].Subject != "quarterly report" {
		t.Fatalf("foreign owner copy leaked: %+v", page.Messages)
	}
	// The foreign account id is outside A's scope, so the filtered page is empty.
	foreignScope := readGet(t, f.handler, "/v1/mail/search?q=deployment+window&account_id="+otherAccount.ID, cookie)
	page = mailListPage{}
	decodeBody(t, foreignScope, &page)
	if len(page.Messages) != 0 {
		t.Fatalf("foreign account scope leaked: %+v", page.Messages)
	}
	wantProblem(t, readGet(t, f.handler, "/v1/mail/search?q=x&tracker=1", cookie),
		http.StatusBadRequest, "invalid_request")
	miss := readGet(t, f.handler, "/v1/mail/search?q=zzznomatchzzz", cookie)
	if miss.Code != http.StatusOK || !strings.Contains(miss.Body.String(), `"messages":[]`) {
		t.Fatalf("miss: %d %s", miss.Code, miss.Body.String())
	}
	// Search pages use the same stable keyset cursor as the lists.
	for i := range 21 {
		insert(account, "<s-page-"+string(rune('a'+i))+"@example.invalid>",
			"bulk subject", "bulk haystack "+string(rune('a'+i)), f.baseTime.Add(time.Duration(3+i)*time.Minute))
	}
	first := readGet(t, f.handler, "/v1/mail/search?q=haystack", cookie)
	page = mailListPage{}
	decodeBody(t, first, &page)
	if len(page.Messages) != 20 || page.NextCursor == "" {
		t.Fatalf("search first page: %d messages", len(page.Messages))
	}
	// A matching record arriving after the cursor never shifts the next page.
	newer := f.mailInput(account.ID, "<s-page-new@example.invalid>", "newer bulk",
		"newer haystack", f.baseTime.Add(60*time.Minute))
	if _, err := f.mail.InsertMessage(ctx, f.ownerID, newer, f.baseTime.Add(60*time.Minute)); err != nil {
		t.Fatal(err)
	}
	secondPage := readGet(t, f.handler, "/v1/mail/search?q=haystack&cursor="+page.NextCursor, cookie)
	page = mailListPage{}
	decodeBody(t, secondPage, &page)
	if len(page.Messages) != 1 || page.Messages[0].Subject != "bulk subject" || page.NextCursor != "" {
		t.Fatalf("search second page shifted: %+v", page.Messages)
	}
}

func TestSMSReadEndpointsAndIsolation(t *testing.T) {
	f := newReadFixture(t)
	other := newReadFixture(t)
	ctx := context.Background()
	cookie, _ := ownerSession(t, f)
	device := f.device(t, "Synthetic Agent A")
	ids := make([]string, 0, 3)
	for i := range 3 {
		// received_at is the server receive time: advance it per record so the
		// keyset order is deterministic.
		at := f.baseTime.Add(time.Duration(i) * time.Minute)
		input := f.smsInput(device.ID, "evt-"+string(rune('a'+i)), "synthetic sms body "+string(rune('a'+i)), at)
		if status, err := f.sms.InsertMessage(ctx, f.ownerID, input, at); err != nil || status != smsstore.StatusAccepted {
			t.Fatalf("sms import: %s, %v", status, err)
		}
		ids = append(ids, f.smsID(t, input.RemoteEventID))
	}
	// Owner B pairs its own device and stores its own record.
	otherDevice := other.device(t, "Synthetic Agent B")
	otherInput := other.smsInput(otherDevice.ID, "evt-foreign", "other owner synthetic sms body", other.baseTime)
	if _, err := other.sms.InsertMessage(ctx, other.ownerID, otherInput, other.baseTime); err != nil {
		t.Fatal(err)
	}

	full := readGet(t, f.handler, "/v1/sms/messages", cookie)
	if full.Code != http.StatusOK {
		t.Fatalf("sms list: %d %s", full.Code, full.Body.String())
	}
	var page smsListPage
	decodeBody(t, full, &page)
	if len(page.Messages) != 3 || page.Messages[0].ID != ids[2] || page.NextCursor != "" {
		t.Fatalf("sms list order: %+v", page.Messages)
	}
	if page.Messages[0].Sender != "10086" || page.Messages[0].Recipient != "1069000000000" ||
		page.Messages[0].Direction != "inbound" || page.Messages[0].DeviceID != device.ID {
		t.Fatalf("sms fields: %+v", page.Messages[0])
	}
	byDevice := readGet(t, f.handler, "/v1/sms/messages?device_id="+device.ID, cookie)
	page = smsListPage{}
	decodeBody(t, byDevice, &page)
	if len(page.Messages) != 3 {
		t.Fatalf("device filter: %+v", page.Messages)
	}
	// An unknown device id is just an empty filter result, never foreign data.
	emptyScope := readGet(t, f.handler, "/v1/sms/messages?device_id="+otherDevice.ID, cookie)
	page = smsListPage{}
	decodeBody(t, emptyScope, &page)
	if len(page.Messages) != 0 {
		t.Fatalf("unknown device filter leaked: %+v", page.Messages)
	}
	// Stable pagination.
	first := readGet(t, f.handler, "/v1/sms/messages?limit=1", cookie)
	page = smsListPage{}
	decodeBody(t, first, &page)
	if len(page.Messages) != 1 || page.Messages[0].ID != ids[2] || page.NextCursor == "" {
		t.Fatalf("sms first page: %+v", page.Messages)
	}
	newer := f.smsInput(device.ID, "evt-new", "newer synthetic sms body", f.baseTime.Add(10*time.Minute))
	if _, err := f.sms.InsertMessage(ctx, f.ownerID, newer, f.baseTime.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	second := readGet(t, f.handler, "/v1/sms/messages?limit=1&cursor="+page.NextCursor, cookie)
	page = smsListPage{}
	decodeBody(t, second, &page)
	if len(page.Messages) != 1 || page.Messages[0].ID != ids[1] {
		t.Fatalf("sms second page shifted: %+v", page.Messages)
	}
	third := readGet(t, f.handler, "/v1/sms/messages?limit=1&cursor="+page.NextCursor, cookie)
	page = smsListPage{}
	decodeBody(t, third, &page)
	if len(page.Messages) != 1 || page.Messages[0].ID != ids[0] || page.NextCursor != "" {
		t.Fatalf("sms third page: %+v", page.Messages)
	}
	wantProblem(t, readGet(t, f.handler, "/v1/sms/messages?cursor=broken", cookie),
		http.StatusBadRequest, "invalid_request")

	got := readGet(t, f.handler, "/v1/sms/messages/"+ids[0], cookie)
	if got.Code != http.StatusOK {
		t.Fatalf("sms detail: %d %s", got.Code, got.Body.String())
	}
	var detail smsSummary
	decodeBody(t, got, &detail)
	if detail.ID != ids[0] || detail.Body != "synthetic sms body a" {
		t.Fatalf("sms detail: %+v", detail)
	}
	wantProblem(t, readGet(t, f.handler, "/v1/sms/messages/00000000-0000-4000-8000-000000000000", cookie),
		http.StatusNotFound, "not_found")
	// Owner B's record is 404 for owner A.
	wantProblem(t, readGet(t, f.handler, "/v1/sms/messages/"+other.smsID(t, "evt-foreign"), cookie),
		http.StatusNotFound, "not_found")

	var searched smsListPage
	senderHit := readGet(t, f.handler, "/v1/sms/search?q=10086", cookie)
	decodeBody(t, senderHit, &searched)
	// All four owner records (three plus the newer pagination record) share the sender.
	if len(searched.Messages) != 4 {
		t.Fatalf("sender search: %+v", searched.Messages)
	}
	bodyHit := readGet(t, f.handler, "/v1/sms/search?q=sms+body+b", cookie)
	searched = smsListPage{}
	decodeBody(t, bodyHit, &searched)
	if len(searched.Messages) != 1 || searched.Messages[0].ID != ids[1] {
		t.Fatalf("body search: %+v", searched.Messages)
	}
	foreignHit := readGet(t, f.handler, "/v1/sms/search?q=other+owner+synthetic", cookie)
	searched = smsListPage{}
	decodeBody(t, foreignHit, &searched)
	if len(searched.Messages) != 0 {
		t.Fatalf("foreign body searchable: %+v", searched.Messages)
	}
	deviceScoped := readGet(t, f.handler, "/v1/sms/search?q=sms+body&device_id="+otherDevice.ID, cookie)
	searched = smsListPage{}
	decodeBody(t, deviceScoped, &searched)
	if len(searched.Messages) != 0 {
		t.Fatalf("foreign device search scope leaked: %+v", searched.Messages)
	}
	for _, missing := range []string{"", "?q=", "?q=%20", "?device_id=" + device.ID} {
		wantProblem(t, readGet(t, f.handler, "/v1/sms/search"+missing, cookie),
			http.StatusBadRequest, "invalid_request")
	}
	wantProblem(t, readGet(t, f.handler, "/v1/sms/search?q=x&bogus=1", cookie),
		http.StatusBadRequest, "invalid_request")
}

func TestReadEndpointsFailClosedOnTamperedCiphertext(t *testing.T) {
	f := newReadFixture(t)
	ctx := context.Background()
	cookie, _ := ownerSession(t, f)
	account := f.mailAccount(t)
	input := f.mailInput(account.ID, "<tamper@example.invalid>", "tamper subject",
		"synthetic tamper body", f.baseTime)
	if _, err := f.mail.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	messageID := f.mailID(t, input.RemoteMessageID)
	tamperCiphertext(t, f.db, "mail_messages", "body_ciphertext", "message_id", messageID)
	listPath := "/v1/mail/accounts/" + account.ID + "/messages"
	wantProblem(t, readGet(t, f.handler, "/v1/mail/messages/"+messageID, cookie),
		http.StatusInternalServerError, "decryption_failed")
	for _, path := range []string{listPath, "/v1/mail/search?q=synthetic"} {
		recorder := readGet(t, f.handler, path, cookie)
		wantProblem(t, recorder, http.StatusInternalServerError, "decryption_failed")
		if strings.Contains(recorder.Body.String(), "synthetic") || strings.Contains(recorder.Body.String(), "messages") {
			t.Fatalf("failed %s returned partial data: %s", path, recorder.Body.String())
		}
	}
	device := f.device(t, "Synthetic Agent C")
	smsEntry := f.smsInput(device.ID, "evt-tamper", "synthetic tamper sms", f.baseTime)
	if _, err := f.sms.InsertMessage(ctx, f.ownerID, smsEntry, f.baseTime); err != nil {
		t.Fatal(err)
	}
	smsRecord := f.smsID(t, smsEntry.RemoteEventID)
	tamperCiphertext(t, f.db, "sms_messages", "body_ciphertext", "sms_id", smsRecord)
	wantProblem(t, readGet(t, f.handler, "/v1/sms/messages/"+smsRecord, cookie),
		http.StatusInternalServerError, "decryption_failed")
	for _, path := range []string{"/v1/sms/messages", "/v1/sms/search?q=synthetic"} {
		recorder := readGet(t, f.handler, path, cookie)
		wantProblem(t, recorder, http.StatusInternalServerError, "decryption_failed")
		if strings.Contains(recorder.Body.String(), "synthetic") || strings.Contains(recorder.Body.String(), "messages") {
			t.Fatalf("failed %s returned partial data: %s", path, recorder.Body.String())
		}
	}
}

func tamperCiphertext(t *testing.T, db *sql.DB, table, column, idColumn, id string) {
	t.Helper()
	var stored []byte
	if err := db.QueryRow("SELECT "+column+" FROM "+table+" WHERE "+idColumn+" = ?", id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	stored[len(stored)-1] ^= 1
	if _, err := db.Exec("UPDATE "+table+" SET "+column+" = ? WHERE "+idColumn+" = ?", stored, id); err != nil {
		t.Fatal(err)
	}
}

func TestReadQueryExemptionKeepsOtherRoutesStrict(t *testing.T) {
	f := newReadFixture(t)
	cookie, _ := ownerSession(t, f)
	// Existing business routes keep rejecting every query string.
	for _, path := range []string{"/v1/mail-accounts?cursor=1", "/v1/synthetic-events?limit=5", "/v1/devices?page=1"} {
		wantProblem(t, readGet(t, f.handler, path, cookie), http.StatusBadRequest, "invalid_request")
	}
	// The read surface only accepts its documented parameters.
	wantProblem(t, readGet(t, f.handler, "/v1/mail/search?q=x&tracker=1", cookie),
		http.StatusBadRequest, "invalid_request")
	wantProblem(t, readGet(t, f.handler, "/v1/sms/messages/00000000-0000-4000-8000-000000000000?format=html", cookie),
		http.StatusBadRequest, "invalid_request")
}
