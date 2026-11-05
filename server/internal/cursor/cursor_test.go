package cursor

import (
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	value, err := Encode(1730800000123, "018f3c2e-9a41-4b6e-8f7c-3f5a2b1d0e99")
	if err != nil {
		t.Fatal(err)
	}
	atMs, id, err := Decode(value)
	if err != nil || atMs != 1730800000123 || id != "018f3c2e-9a41-4b6e-8f7c-3f5a2b1d0e99" {
		t.Fatalf("round trip: %d %q %v", atMs, id, err)
	}
}

func TestDecodeEdgeCases(t *testing.T) {
	atMs, id, err := Decode("")
	if err != nil || atMs != 0 || id != "" {
		t.Fatalf("empty cursor must start a first page: %d %q %v", atMs, id, err)
	}
	for _, invalid := range []string{"!!!", "e30", "eyJpZCI6IiJ9"} {
		if _, _, err := Decode(invalid); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid cursor %q accepted: %v", invalid, err)
		}
	}
	if _, err := Encode(1, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty identifier accepted: %v", err)
	}
}
