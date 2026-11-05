package randid

import (
	"regexp"
	"testing"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewFormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool, 100)
	for range 100 {
		id, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if !uuidPattern.MatchString(id) {
			t.Fatalf("identifier format invalid: %s", id)
		}
		if seen[id] {
			t.Fatalf("identifier repeated: %s", id)
		}
		seen[id] = true
	}
}
