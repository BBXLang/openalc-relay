package secrets

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestKeyFileAndAuthenticatedEnvelope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	if err := CreateFile(path); err != nil {
		t.Fatal(err)
	}
	if err := CreateFile(path); err == nil {
		t.Fatal("keygen must not overwrite an existing key")
	}
	keys, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte("owner/device/event/type")
	plaintext := []byte("synthetic fixture only")
	envelope, err := keys.Seal(aad, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(envelope.Ciphertext, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}
	opened, err := keys.Open(aad, envelope)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("round trip failed: %v", err)
	}
	if _, err := keys.Open([]byte("other owner"), envelope); err == nil {
		t.Fatal("AAD substitution accepted")
	}
	envelope.Ciphertext[0] ^= 1
	if _, err := keys.Open(aad, envelope); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestKeyRotationKeepsOldDecryptKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	if err := CreateFile(path); err != nil {
		t.Fatal(err)
	}
	old, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := old.Seal([]byte("context"), []byte("value"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file keyFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	file.Active = "k2"
	file.Keys["k2"] = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	data, err = json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	rotated, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := rotated.Open([]byte("context"), envelope)
	if err != nil || string(opened) != "value" {
		t.Fatalf("old key unavailable after rotation: %v", err)
	}
	newEnvelope, err := rotated.Seal([]byte("context"), []byte("new"))
	if err != nil || newEnvelope.KeyID != "k2" {
		t.Fatalf("new key not active: %v", err)
	}
}

func TestInvalidKeyFileFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(path, []byte(`{"active":"k1","keys":{"k1":"not-a-key"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("malformed key accepted")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadFile(path); err == nil {
			t.Fatal("group-readable key file accepted")
		}
	}
}
