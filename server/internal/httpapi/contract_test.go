package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Verify every advertised response against the running handler, including body suppression.
// This is an integration consistency check, not a complete OpenAPI schema validator.
func TestImplementedOpenAPIContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "packages", "openapi", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]struct {
					Schema struct {
						Ref string `json:"$ref"`
					} `json:"schema"`
				} `json:"content"`
			} `json:"responses"`
		} `json:"paths"`
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Const string `json:"const"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	expectedMethods := map[string][]string{
		"/healthz": {"get", "head"}, "/readyz": {"get", "head"},
		"/v1/session":  {"get", "post", "delete"},
		"/v1/pairings": {"post"}, "/v1/pairings/{pairing_id}": {"get"},
		"/v1/pairings/{pairing_id}/requests": {"post"},
		"/v1/pairings/{pairing_id}/approve":  {"post"},
		"/v1/pairings/{pairing_id}/exchange": {"post"},
		"/v1/devices":                        {"get"}, "/v1/devices/{device_id}/revoke": {"post"},
		"/v1/devices/{device_id}/synthetic-pings":        {"post"},
		"/v1/synthetic-events":                           {"get"},
		"/v1/synthetic-pings/{ping_id}":                  {"get"},
		"/v1/agent/synthetic-sync":                       {"post"},
		"/v1/mail-accounts":                              {"get", "post"},
		"/v1/mail-accounts/{account_id}":                 {"delete"},
		"/v1/mail-accounts/{account_id}/sync":            {"post"},
		"/v1/mail-accounts/{account_id}/connection-test": {"post"},
		"/v1/mail/accounts/{account_id}/messages":        {"get"},
		"/v1/mail/messages/{message_id}":                 {"get"},
		"/v1/mail/search":                                {"get"},
		"/v1/sms/messages":                               {"get"},
		"/v1/sms/messages/{sms_id}":                      {"get"},
		"/v1/sms/search":                                 {"get"},
		"/v1/sms/send-commands":                          {"get", "post"},
		"/v1/sms/send-commands/{command_id}":             {"get"},
	}
	if contract.OpenAPI != "3.1.0" || len(contract.Paths) != len(expectedMethods) {
		t.Fatal("unexpected implemented API surface")
	}
	for path, methods := range expectedMethods {
		actual, exists := contract.Paths[path]
		if !exists || len(actual) != len(methods) {
			t.Fatalf("contract methods missing for %s", path)
		}
		for _, method := range methods {
			if actual[method].Responses == nil {
				t.Fatalf("contract response missing for %s %s", method, path)
			}
		}
	}
	for path, methods := range contract.Paths {
		if path != "/healthz" && path != "/readyz" {
			continue // Business route outcomes are exercised with a real owner and device.
		}
		for method, operation := range methods {
			for code, response := range operation.Responses {
				t.Run(method+" "+path+" "+code, func(t *testing.T) {
					handler := NewHandler(func(context.Context) error {
						if code == "503" {
							return errors.New("private detail")
						}
						return nil
					})
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, httptest.NewRequest(strings.ToUpper(method), path, nil))
					if strconv.Itoa(recorder.Code) != code {
						t.Fatalf("documented %s, returned %d", code, recorder.Code)
					}
					if method == "head" {
						if recorder.Body.Len() != 0 {
							t.Fatal("HEAD response contains a body")
						}
						return
					}
					ref := response.Content["application/json"].Schema.Ref
					schema, exists := contract.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
					if !exists {
						t.Fatal("response schema reference is missing")
					}
					var body map[string]string
					if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					if len(body) != 1 || body["status"] != schema.Properties["status"].Const {
						t.Fatalf("response does not match documented status: %s", recorder.Body.String())
					}
				})
			}
		}
	}
}
