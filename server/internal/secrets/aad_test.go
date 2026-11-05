package secrets

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestAADFixedVector(t *testing.T) {
	// Fixed vector: the layout is normative and must not change silently.
	aad, err := AAD(DomainMailMessage, "owner-1", "account-1", "msg-1", "subject")
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte("relay/mail_message/v1\x00owner-1\x00account-1\x00msg-1\x00subject")
	if !bytes.Equal(aad, expected) {
		t.Fatalf("AAD layout changed: %q", aad)
	}
}

func TestAADDomainSeparation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	if err := CreateFile(path); err != nil {
		t.Fatal(err)
	}
	keys, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	coordinates := []struct{ domain, owner, scope, record, field string }{
		{DomainMailMessage, "owner-1", "account-1", "msg-1", "subject"},
		{DomainMailMessage, "owner-1", "account-1", "msg-1", "body"},
		{DomainMailMessage, "owner-1", "account-1", "msg-2", "subject"},
		{DomainMailMessage, "owner-1", "account-2", "msg-1", "subject"},
		{DomainMailMessage, "owner-2", "account-1", "msg-1", "subject"},
		{DomainSMSMessage, "owner-1", "device-1", "sms-1", "body"},
	}
	plaintext := []byte("synthetic fixture only")
	for i, coordinate := range coordinates {
		aad, err := AAD(coordinate.domain, coordinate.owner, coordinate.scope, coordinate.record, coordinate.field)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := keys.Seal(aad, plaintext)
		if err != nil {
			t.Fatal(err)
		}
		// Every other coordinate must fail to open this envelope.
		for j, other := range coordinates {
			if j == i {
				continue
			}
			mismatched, err := AAD(other.domain, other.owner, other.scope, other.record, other.field)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := keys.Open(mismatched, envelope); err == nil {
				t.Fatalf("coordinate %d opened under coordinate %d", i, j)
			}
		}
		opened, err := keys.Open(aad, envelope)
		if err != nil || !bytes.Equal(opened, plaintext) {
			t.Fatalf("coordinate %d round trip failed: %v", i, err)
		}
	}
}

func TestAADRejectsAmbiguousComponents(t *testing.T) {
	for _, component := range []string{"", "a\x00b"} {
		if _, err := AAD(component, "o", "s", "r", "f"); err == nil {
			t.Fatalf("invalid component %q accepted", component)
		}
	}
}

func TestEnvelopeBlobRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	if err := CreateFile(path); err != nil {
		t.Fatal(err)
	}
	keys, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte("relay/test/v1\x00o\x00s\x00r\x00f")
	stored, err := keys.SealToBlob(aad, []byte("value"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := keys.OpenFromBlob(aad, stored)
	if err != nil || string(opened) != "value" {
		t.Fatalf("blob round trip failed: %v", err)
	}
	if _, err := keys.OpenFromBlob([]byte("relay/test/v1\x00o\x00s\x00r\x00other"), stored); err == nil {
		t.Fatal("AAD substitution accepted for stored envelope")
	}
	stored[len(stored)-1] ^= 1
	if _, err := keys.OpenFromBlob(aad, stored); err == nil {
		t.Fatal("tampered stored envelope accepted")
	}
	if _, err := keys.OpenFromBlob(aad, []byte("{not json")); err == nil {
		t.Fatal("malformed stored envelope accepted")
	}
}
