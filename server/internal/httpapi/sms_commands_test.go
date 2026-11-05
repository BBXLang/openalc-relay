package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/OpenALC/relay/server/internal/smsstore"
)

// The send-command surface is the owner half of the RFC-0003 §3 state
// machine. Every test uses synthetic data only: the recipient is the fixed
// synthetic short code 100087 (the same fixture convention as the smsstore
// tests, not a dialable number) and every body text is marked "synthetic".
// Nothing in this file reaches a device or a network: commands stay queued
// because the device claim/report protocol is still Proposed.

const (
	sendRecipient = "100087"              // synthetic short-code fixture value
	sendBody      = "synthetic send body" // marked synthetic by wording
)

// contentFieldNames are the fields that must never appear in any command
// response: message content and the retry key stay server-side.
var contentFieldNames = []string{
	"recipient", "body", "idempotency_key", "recipient_ciphertext", "body_ciphertext",
}

func sendPayload(deviceID, key, body string) string {
	return fmt.Sprintf(`{"device_id":%q,"recipient":%q,"body":%q,"idempotency_key":%q,"confirm":true}`,
		deviceID, sendRecipient, body, key)
}

func postSendCommand(t *testing.T, handler http.Handler, cookie *http.Cookie, csrf, payload string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, mailRequest(http.MethodPost, "/v1/sms/send-commands", payload, cookie, csrf))
	return recorder
}

func getSendCommandPath(t *testing.T, handler http.Handler, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, mailRequest(http.MethodGet, path, "", cookie, ""))
	return recorder
}

// assertNoContent verifies that a response carries neither the synthetic
// content values nor any content-bearing field name, at any nesting depth.
func assertNoContent(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	text := recorder.Body.String()
	if strings.Contains(text, sendRecipient) || strings.Contains(text, sendBody) {
		t.Fatalf("response leaks message content: %s", text)
	}
	var decoded any
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("response body: %v: %s", err, text)
	}
	assertNoContentFields(t, decoded, "response")
}

func assertNoContentFields(t *testing.T, value any, context string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for name, nested := range typed {
			if slices.Contains(contentFieldNames, name) {
				t.Fatalf("%s carries forbidden field %s", context, name)
			}
			assertNoContentFields(t, nested, context)
		}
	case []any:
		for _, nested := range typed {
			assertNoContentFields(t, nested, context)
		}
	}
}

type smsCommandCreatedBody struct {
	CommandID        string `json:"command_id"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
	IdempotentReplay bool   `json:"idempotent_replay"`
}

type smsCommandStateBody struct {
	CommandID  string  `json:"command_id"`
	DeviceID   string  `json:"device_id"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	ResolvedAt *string `json:"resolved_at"`
}

type smsCommandListBody struct {
	Commands   []smsCommandStateBody `json:"commands"`
	NextCursor string                `json:"next_cursor"`
}

func TestSendCommandRequiresSessionConfirmationOriginAndCSRF(t *testing.T) {
	handler := businessTestHandler(t)
	payload := sendPayload("00000000-0000-4000-8000-00000000000a", "idempotency-key-auth-0001", sendBody)

	anonymous := postSendCommand(t, handler, nil, "", payload)
	wantProblem(t, anonymous, http.StatusUnauthorized, "session_required")
	anonymousList := getSendCommandPath(t, handler, nil, "/v1/sms/send-commands")
	wantProblem(t, anonymousList, http.StatusUnauthorized, "session_required")

	cookie, csrf := loginOwner(t, handler)
	missingConfirm := postSendCommand(t, handler, cookie, csrf,
		`{"device_id":"00000000-0000-4000-8000-00000000000a","recipient":"100087",
		"body":"synthetic send body","idempotency_key":"idempotency-key-auth-0002"}`)
	wantProblem(t, missingConfirm, http.StatusBadRequest, "missing_confirmation")
	falseConfirm := postSendCommand(t, handler, cookie, csrf,
		strings.Replace(sendPayload("00000000-0000-4000-8000-00000000000a", "idempotency-key-auth-0003", sendBody),
			`"confirm":true`, `"confirm":false`, 1))
	wantProblem(t, falseConfirm, http.StatusBadRequest, "missing_confirmation")

	missingCSRF := postSendCommand(t, handler, cookie, "", payload)
	wantProblem(t, missingCSRF, http.StatusForbidden, "csrf_rejected")

	crossOriginRequest := mailRequest(http.MethodPost, "/v1/sms/send-commands", payload, cookie, csrf)
	crossOriginRequest.Header.Set("Origin", "https://evil.example")
	crossOrigin := httptest.NewRecorder()
	handler.ServeHTTP(crossOrigin, crossOriginRequest)
	wantProblem(t, crossOrigin, http.StatusForbidden, "origin_rejected")

	withQueryRequest := mailRequest(http.MethodPost, "/v1/sms/send-commands?device_id=x", payload, cookie, csrf)
	withQuery := httptest.NewRecorder()
	handler.ServeHTTP(withQuery, withQueryRequest)
	wantProblem(t, withQuery, http.StatusBadRequest, "invalid_request")

	// Format bounds mirror the immutable schema CHECK: printable ASCII,
	// 16..128 bytes. json.Marshal escapes the control byte into a
	// well-formed JSON string whose decoded value is not printable ASCII,
	// so the rejection comes from the key format check, not JSON parsing.
	escapedKey, err := json.Marshal("idempotency-key-auth-0001" + string(rune(7)))
	if err != nil {
		t.Fatal(err)
	}
	controlChar := postSendCommand(t, handler, cookie, csrf, fmt.Sprintf(
		`{"device_id":"00000000-0000-4000-8000-00000000000a","recipient":%q,
		"body":%q,"idempotency_key":%s,"confirm":true}`,
		sendRecipient, sendBody, string(escapedKey)))
	wantProblem(t, controlChar, http.StatusBadRequest, "invalid_request")
	tooShort := postSendCommand(t, handler, cookie, csrf,
		`{"device_id":"00000000-0000-4000-8000-00000000000a","recipient":"100087",
		"body":"synthetic send body","idempotency_key":"short-key-1","confirm":true}`)
	wantProblem(t, tooShort, http.StatusBadRequest, "invalid_request")
}

func TestSendCommandIdempotencyAndConflictOverHTTP(t *testing.T) {
	f := newReadFixture(t)
	device := f.device(t, "Xiaomi Agent")
	cookie, csrf := ownerSession(t, f)
	key := "idempotency-key-http-0001"

	created := postSendCommand(t, f.handler, cookie, csrf, sendPayload(device.ID, key, sendBody))
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	assertNoContent(t, created)
	var first smsCommandCreatedBody
	decodeBody(t, created, &first)
	if first.CommandID == "" || first.Status != smsstore.StatusQueued || first.IdempotentReplay {
		t.Fatalf("created command body: %+v", first)
	}

	replay := postSendCommand(t, f.handler, cookie, csrf, sendPayload(device.ID, key, sendBody))
	if replay.Code != http.StatusOK {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
	}
	var replayed smsCommandCreatedBody
	decodeBody(t, replay, &replayed)
	if replayed.CommandID != first.CommandID || !replayed.IdempotentReplay ||
		replayed.CreatedAt != first.CreatedAt || replayed.Status != smsstore.StatusQueued {
		t.Fatalf("idempotent replay mismatch: %+v vs %+v", replayed, first)
	}
	assertNoContent(t, replay)

	other := f.device(t, "Second Agent")
	sameKeyDifferentBody := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload(device.ID, key, "different synthetic body"))
	wantProblem(t, sameKeyDifferentBody, http.StatusConflict, "idempotency_conflict")
	sameKeyDifferentDevice := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload(other.ID, key, sendBody))
	wantProblem(t, sameKeyDifferentDevice, http.StatusConflict, "idempotency_conflict")

	detail := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands/"+first.CommandID)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body.String())
	}
	assertNoContent(t, detail)
	var state smsCommandStateBody
	decodeBody(t, detail, &state)
	if state.CommandID != first.CommandID || state.DeviceID != device.ID ||
		state.Status != smsstore.StatusQueued || state.CreatedAt != first.CreatedAt ||
		state.ResolvedAt != nil {
		t.Fatalf("conflict mutated the stored command: %+v vs %+v", state, first)
	}
	var rawState map[string]any
	if err := json.Unmarshal(detail.Body.Bytes(), &rawState); err != nil {
		t.Fatal(err)
	}
	want := []string{"command_id", "device_id", "status", "created_at"}
	if len(rawState) != len(want) {
		t.Fatalf("detail fields: %v", rawState)
	}
	for _, name := range want {
		if _, present := rawState[name]; !present {
			t.Fatalf("detail missing %s: %v", name, rawState)
		}
	}
	var keyCount int
	if err := f.db.QueryRow(`SELECT count(*) FROM sms_send_commands
		WHERE idempotency_key = ?`, key).Scan(&keyCount); err != nil {
		t.Fatal(err)
	}
	if keyCount != 1 {
		t.Fatalf("command rows for key: %d", keyCount)
	}
	missing := getSendCommandPath(t, f.handler, cookie,
		"/v1/sms/send-commands/00000000-0000-4000-8000-000000000000")
	wantProblem(t, missing, http.StatusNotFound, "not_found")
}

func TestSendCommandListFiltersAndPagination(t *testing.T) {
	f := newReadFixture(t)
	device := f.device(t, "Xiaomi Agent")
	other := f.device(t, "Second Agent")
	cookie, csrf := ownerSession(t, f)

	for i, target := range []string{device.ID, other.ID} {
		created := postSendCommand(t, f.handler, cookie, csrf,
			sendPayload(target, fmt.Sprintf("idempotency-key-list-%04d", i), sendBody))
		if created.Code != http.StatusCreated {
			t.Fatalf("create %d: %d %s", i, created.Code, created.Body.String())
		}
	}

	full := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands")
	if full.Code != http.StatusOK {
		t.Fatalf("list: %d %s", full.Code, full.Body.String())
	}
	assertNoContent(t, full)
	var page smsCommandListBody
	decodeBody(t, full, &page)
	if len(page.Commands) != 2 || page.NextCursor != "" {
		t.Fatalf("full list: %+v", page)
	}

	byDevice := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands?device_id="+device.ID)
	var devicePage smsCommandListBody
	decodeBody(t, byDevice, &devicePage)
	if byDevice.Code != http.StatusOK || len(devicePage.Commands) != 1 ||
		devicePage.Commands[0].DeviceID != device.ID {
		t.Fatalf("device filter: %d %+v", byDevice.Code, devicePage)
	}
	unknownDevice := getSendCommandPath(t, f.handler, cookie,
		"/v1/sms/send-commands?device_id=00000000-0000-4000-8000-000000000000")
	var emptyPage smsCommandListBody
	decodeBody(t, unknownDevice, &emptyPage)
	if unknownDevice.Code != http.StatusOK || len(emptyPage.Commands) != 0 {
		t.Fatalf("unknown device filter: %d %+v", unknownDevice.Code, emptyPage)
	}

	queued := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands?status=queued")
	var queuedPage smsCommandListBody
	decodeBody(t, queued, &queuedPage)
	if queued.Code != http.StatusOK || len(queuedPage.Commands) != 2 {
		t.Fatalf("queued filter: %d %+v", queued.Code, queuedPage)
	}
	failed := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands?status=failed")
	var failedPage smsCommandListBody
	decodeBody(t, failed, &failedPage)
	if failed.Code != http.StatusOK || failedPage.Commands == nil || len(failedPage.Commands) != 0 {
		t.Fatalf("failed filter: %d %+v", failed.Code, failedPage)
	}
	badStatus := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands?status=sent")
	wantProblem(t, badStatus, http.StatusBadRequest, "invalid_request")
	badParam := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands?recipient=100087")
	wantProblem(t, badParam, http.StatusBadRequest, "invalid_request")

	firstPage := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands?limit=1")
	var pageOne smsCommandListBody
	decodeBody(t, firstPage, &pageOne)
	if firstPage.Code != http.StatusOK || len(pageOne.Commands) != 1 || pageOne.NextCursor == "" {
		t.Fatalf("first page: %d %+v", firstPage.Code, pageOne)
	}
	secondPage := getSendCommandPath(t, f.handler, cookie,
		"/v1/sms/send-commands?limit=1&cursor="+pageOne.NextCursor)
	var pageTwo smsCommandListBody
	decodeBody(t, secondPage, &pageTwo)
	if secondPage.Code != http.StatusOK || len(pageTwo.Commands) != 1 || pageTwo.NextCursor != "" {
		t.Fatalf("second page: %d %+v", secondPage.Code, pageTwo)
	}
	seen := []string{pageOne.Commands[0].CommandID, pageTwo.Commands[0].CommandID}
	if seen[0] == seen[1] {
		t.Fatalf("cursor repeated a command: %+v", seen)
	}
	for _, command := range seen {
		if command != page.Commands[0].CommandID && command != page.Commands[1].CommandID {
			t.Fatalf("cursor page outside full list: %+v vs %+v", seen, page.Commands)
		}
	}
}

func TestSendCommandQueueLimit(t *testing.T) {
	f := newReadFixture(t)
	device := f.device(t, "Xiaomi Agent")
	cookie, csrf := ownerSession(t, f)
	for i := range smsstore.MaxQueuedCommandsPerOwner {
		created := postSendCommand(t, f.handler, cookie, csrf,
			sendPayload(device.ID, fmt.Sprintf("idempotency-key-queue-%04d", i), sendBody))
		if created.Code != http.StatusCreated {
			t.Fatalf("queued command %d: %d %s", i, created.Code, created.Body.String())
		}
	}
	overLimit := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload(device.ID, "idempotency-key-queue-9999", sendBody))
	wantProblem(t, overLimit, http.StatusTooManyRequests, "queue_limit")
	// A rejected request must not leave a row behind.
	var total int
	if err := f.db.QueryRow("SELECT count(*) FROM sms_send_commands").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != smsstore.MaxQueuedCommandsPerOwner {
		t.Fatalf("stored commands: %d", total)
	}
}

func TestSendCommandDeviceScoping(t *testing.T) {
	f := newReadFixture(t)
	device := f.device(t, "Xiaomi Agent")
	cookie, csrf := ownerSession(t, f)

	unknown := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload("00000000-0000-4000-8000-000000000000", "idempotency-key-scope-0001", sendBody))
	wantProblem(t, unknown, http.StatusNotFound, "not_found")

	if err := f.pairings.RevokeDevice(context.Background(), f.ownerID, device.ID, f.baseTime); err != nil {
		t.Fatal(err)
	}
	revoked := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload(device.ID, "idempotency-key-scope-0002", sendBody))
	wantProblem(t, revoked, http.StatusForbidden, "device_revoked")

	var total int
	if err := f.db.QueryRow("SELECT count(*) FROM sms_send_commands").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("rejected commands stored: %d", total)
	}
}

func TestSendCommandRequestRateLimit(t *testing.T) {
	f := newReadFixture(t)
	f.device(t, "Xiaomi Agent")
	cookie, csrf := ownerSession(t, f)
	// Valid-format requests for a device the owner does not have: each one
	// is answered 404 by the store but still consumes request budget, which
	// is what the limiter is for.
	for i := range sendCommandRateLimit {
		denied := postSendCommand(t, f.handler, cookie, csrf,
			sendPayload("00000000-0000-4000-8000-000000000000", fmt.Sprintf("idempotency-key-ratel-%04d", i), sendBody))
		if denied.Code != http.StatusNotFound {
			t.Fatalf("request %d: %d %s", i, denied.Code, denied.Body.String())
		}
	}
	overLimit := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload("00000000-0000-4000-8000-000000000000", "idempotency-key-ratel-9999", sendBody))
	wantProblem(t, overLimit, http.StatusTooManyRequests, "rate_limited")
}

func TestSendCommandOwnerIsolation(t *testing.T) {
	ownerA := newReadFixture(t)
	ownerB := newReadFixture(t)
	device := ownerA.device(t, "Xiaomi Agent")
	cookie, csrf := ownerSession(t, ownerA)
	created := postSendCommand(t, ownerA.handler, cookie, csrf,
		sendPayload(device.ID, "idempotency-key-iso-000001", sendBody))
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var body smsCommandCreatedBody
	decodeBody(t, created, &body)

	cookieB, _ := ownerSession(t, ownerB)
	foreign := getSendCommandPath(t, ownerB.handler, cookieB, "/v1/sms/send-commands/"+body.CommandID)
	wantProblem(t, foreign, http.StatusNotFound, "not_found")
	listB := getSendCommandPath(t, ownerB.handler, cookieB, "/v1/sms/send-commands")
	var pageB smsCommandListBody
	decodeBody(t, listB, &pageB)
	if listB.Code != http.StatusOK || len(pageB.Commands) != 0 {
		t.Fatalf("foreign owner list: %d %+v", listB.Code, pageB)
	}
}

func TestSendCommandAuditMetadataOnly(t *testing.T) {
	f := newReadFixture(t)
	device := f.device(t, "Xiaomi Agent")
	cookie, csrf := ownerSession(t, f)
	created := make([]string, 0, 2)
	for i := range 2 {
		response := postSendCommand(t, f.handler, cookie, csrf,
			sendPayload(device.ID, fmt.Sprintf("idempotency-key-audit-%04d", i), sendBody))
		if response.Code != http.StatusCreated {
			t.Fatalf("create %d: %d %s", i, response.Code, response.Body.String())
		}
		var body smsCommandCreatedBody
		decodeBody(t, response, &body)
		created = append(created, body.CommandID)
	}
	// An idempotent replay is not a new state change and must not audit.
	replay := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload(device.ID, "idempotency-key-audit-0000", sendBody))
	if replay.Code != http.StatusOK {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
	}

	rows, err := f.db.Query(`SELECT actor_kind, actor_id, subject_kind, subject_id, detail
		FROM audit_entries WHERE action = 'sms.command.create'
		ORDER BY created_at_ms, entry_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	subjects := make([]string, 0)
	for rows.Next() {
		var actorKind, actorID, subjectKind, subjectID, detail string
		if err := rows.Scan(&actorKind, &actorID, &subjectKind, &subjectID, &detail); err != nil {
			t.Fatal(err)
		}
		if actorKind != "owner_session" || actorID != f.ownerID {
			t.Fatalf("audit actor: %s/%s", actorKind, actorID)
		}
		if subjectKind != "sms_command" {
			t.Fatalf("audit subject kind: %s", subjectKind)
		}
		if strings.Contains(detail, sendRecipient) || strings.Contains(detail, sendBody) {
			t.Fatalf("audit detail leaks payload: %s", detail)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(detail), &decoded); err != nil {
			t.Fatalf("audit detail: %v: %s", err, detail)
		}
		for _, name := range contentFieldNames {
			if _, present := decoded[name]; present {
				t.Fatalf("audit detail carries %s: %s", name, detail)
			}
		}
		if decoded["device_id"] != device.ID || decoded["status"] != smsstore.StatusQueued {
			t.Fatalf("audit detail metadata: %s", detail)
		}
		subjects = append(subjects, subjectID)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(subjects, created) {
		t.Fatalf("audit subjects: %v vs %v", subjects, created)
	}
}

func TestSendCommandStaysQueuedWithoutDeviceProtocol(t *testing.T) {
	f := newReadFixture(t)
	device := f.device(t, "Xiaomi Agent")
	cookie, csrf := ownerSession(t, f)
	created := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload(device.ID, "idempotency-key-queued-001", sendBody))
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var body smsCommandCreatedBody
	decodeBody(t, created, &body)

	// Replays and repeated reads are the only paths in this slice; the
	// command must still be queued afterwards, with no timestamp fabricated
	// for a transition that never happened.
	if replay := postSendCommand(t, f.handler, cookie, csrf,
		sendPayload(device.ID, "idempotency-key-queued-001", sendBody)); replay.Code != http.StatusOK {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
	}
	detail := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands/"+body.CommandID)
	var state smsCommandStateBody
	decodeBody(t, detail, &state)
	if state.Status != smsstore.StatusQueued || state.ResolvedAt != nil {
		t.Fatalf("command left queued without a device protocol: %+v", state)
	}
	var rawState map[string]any
	if err := json.Unmarshal(detail.Body.Bytes(), &rawState); err != nil {
		t.Fatal(err)
	}
	for _, phantom := range []string{"claimed_at", "system_sent_at", "resolved_at"} {
		if _, present := rawState[phantom]; present {
			t.Fatalf("detail fabricates transition timestamp %s: %v", phantom, rawState)
		}
	}
	// The device claim/report endpoints are RFC-Proposed and must not exist
	// yet: any transition attempt under this prefix is not found.
	claimRequest := mailRequest(http.MethodPost,
		"/v1/sms/send-commands/"+body.CommandID+"/claim", "", cookie, csrf)
	claim := httptest.NewRecorder()
	f.handler.ServeHTTP(claim, claimRequest)
	wantProblem(t, claim, http.StatusNotFound, "not_found")
	claimedFilter := getSendCommandPath(t, f.handler, cookie, "/v1/sms/send-commands?status=claimed")
	var claimedPage smsCommandListBody
	decodeBody(t, claimedFilter, &claimedPage)
	if claimedFilter.Code != http.StatusOK || len(claimedPage.Commands) != 0 {
		t.Fatalf("claimed filter: %d %+v", claimedFilter.Code, claimedPage)
	}
}
