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
	if contract.OpenAPI != "3.1.0" || len(contract.Paths) != 3 {
		t.Fatal("unexpected implemented API surface")
	}
	methods := contract.Paths["/v1/session"]
	if len(methods) != 3 || methods["get"].Responses == nil ||
		methods["post"].Responses == nil || methods["delete"].Responses == nil {
		t.Fatal("owner session methods missing from contract")
	}
	for path, methods := range contract.Paths {
		if path == "/v1/session" {
			continue // Session outcomes are exercised with a real owner in session_test.go.
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
