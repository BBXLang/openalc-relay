package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenALC/relay/server/internal/synthetic"
)

func TestSyntheticRoutesEnforceOwnerAndDeviceBoundaries(t *testing.T) {
	handler := businessTestHandler(t)
	request := func(method, path, body string, cookie *http.Cookie, csrf, bearer string) *http.Request {
		t.Helper()
		r := httptest.NewRequest(method, "https://relay.test"+path, strings.NewReader(body))
		r.Host = "relay.test"
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		if method == http.MethodPost && cookie != nil {
			r.Header.Set("Origin", "https://relay.test")
		}
		return r
	}
	call := func(r *http.Request, want int, target any) {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: got %d want %d: %s", r.Method, r.URL.Path, w.Code, want, w.Body.String())
		}
		if target != nil && json.Unmarshal(w.Body.Bytes(), target) != nil {
			t.Fatalf("invalid response from %s", r.URL.Path)
		}
	}
	anonymous := request(http.MethodPost, "/v1/pairings", "", nil, "", "")
	anonymous.Header.Set("Origin", "https://relay.test")
	call(anonymous, http.StatusUnauthorized, nil)
	var login struct {
		CSRFToken string `json:"csrf_token"`
	}
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, sessionRequest(http.MethodPost, `{"password":"local owner passphrase"}`))
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("owner login: %d", loginResponse.Code)
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	cookie := loginResponse.Result().Cookies()[0]
	call(request(http.MethodPost, "/v1/pairings", "", cookie, "", ""), http.StatusForbidden, nil)
	chunked := request(http.MethodPost, "/v1/pairings", `{}`, cookie, login.CSRFToken, "")
	chunked.ContentLength = -1
	call(chunked, http.StatusBadRequest, nil)
	var pair synthetic.Pairing
	call(request(http.MethodPost, "/v1/pairings", "", cookie, login.CSRFToken, ""), http.StatusCreated, &pair)
	if pair.ID == "" {
		t.Fatal("pairing ID missing")
	}
	var claim synthetic.Request
	call(request(http.MethodPost, "/v1/pairings/"+pair.ID+"/requests", `{"device_label":"HarmonyOS emulator"}`, nil, "", ""), http.StatusCreated, &claim)
	if claim.ClaimToken == "" || claim.ComparisonCode == "" {
		t.Fatal("pairing claim incomplete")
	}
	call(request(http.MethodPost, "/v1/pairings/"+pair.ID+"/exchange",
		`{"request_id":"`+claim.ID+`","claim_token":"`+claim.ClaimToken+`"}`, nil, "", ""), http.StatusUnauthorized, nil)
	call(request(http.MethodPost, "/v1/pairings/"+pair.ID+"/approve",
		`{"request_id":"`+claim.ID+`","comparison_code":"000000"}`, cookie, login.CSRFToken, ""), http.StatusForbidden, nil)
	call(request(http.MethodPost, "/v1/pairings/"+pair.ID+"/approve",
		`{"request_id":"`+claim.ID+`","comparison_code":"`+claim.ComparisonCode+`"}`, cookie, login.CSRFToken, ""), http.StatusNoContent, nil)
	var device synthetic.Device
	call(request(http.MethodPost, "/v1/pairings/"+pair.ID+"/exchange",
		`{"request_id":"`+claim.ID+`","claim_token":"`+claim.ClaimToken+`"}`, nil, "", ""), http.StatusCreated, &device)
	if device.DeviceToken == "" || device.ID == "" {
		t.Fatal("device exchange incomplete")
	}
	var ping synthetic.Ping
	call(request(http.MethodPost, "/v1/devices/"+device.ID+"/synthetic-pings", "", cookie, login.CSRFToken, ""), http.StatusCreated, &ping)
	eventID := "12345678-1234-4234-8234-123456789abc"
	var synced synthetic.SyncResult
	call(request(http.MethodPost, "/v1/agent/synthetic-sync",
		`{"client_event_id":"`+eventID+`"}`, nil, "", device.DeviceToken), http.StatusOK, &synced)
	if synced.EventStatus != "accepted" || len(synced.Pings) != 1 || synced.Pings[0].ID != ping.ID {
		t.Fatalf("synthetic exchange incomplete: %+v", synced)
	}
	call(request(http.MethodPost, "/v1/agent/synthetic-sync", `{"ack_ping_id":"`+ping.ID+`"}`,
		nil, "", device.DeviceToken), http.StatusOK, &synced)
	if len(synced.Pings) != 0 {
		t.Fatalf("acknowledged ping redelivered: %+v", synced)
	}
	var listed struct {
		Events []synthetic.Event `json:"events"`
	}
	call(request(http.MethodGet, "/v1/synthetic-events", "", cookie, "", ""), http.StatusOK, &listed)
	if len(listed.Events) != 1 || listed.Events[0].ClientEventID != eventID {
		t.Fatalf("owner event list: %+v", listed)
	}
	call(request(http.MethodGet, "/v1/synthetic-events", "", nil, "", device.DeviceToken), http.StatusUnauthorized, nil)
	call(request(http.MethodPost, "/v1/agent/synthetic-sync", `{}`, cookie, "", device.DeviceToken), http.StatusUnauthorized, nil)
	call(request(http.MethodPost, "/v1/devices/"+device.ID+"/revoke", "", cookie, login.CSRFToken, ""), http.StatusNoContent, nil)
	call(request(http.MethodPost, "/v1/agent/synthetic-sync", `{}`, nil, "", device.DeviceToken), http.StatusUnauthorized, nil)
}
