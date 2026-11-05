// Package secrets manages the server's file-backed encryption keys and envelopes.
package secrets

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
)

const maxKeyFileBytes = 16 << 10

var keyIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

type keyFile struct {
	Active string            `json:"active"`
	Keys   map[string]string `json:"keys"`
}

type Keyring struct {
	active string
	keys   map[string][32]byte
}

// Envelope is an authenticated ciphertext. Its AAD is supplied by the caller,
// binding the ciphertext to the owner, device, record and payload type.
type Envelope struct {
	Version    int    `json:"version"`
	KeyID      string `json:"key_id"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// CreateFile creates a new keyring without overwriting an existing key file.
func CreateFile(path string) error {
	if path == "" {
		return errors.New("key file path is required")
	}
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return fmt.Errorf("generate encryption key: %w", err)
	}
	data, err := json.Marshal(keyFile{
		Active: "k1", Keys: map[string]string{"k1": base64.StdEncoding.EncodeToString(key[:])},
	})
	if err != nil {
		return fmt.Errorf("encode key file: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	if _, err = file.Write(append(data, '\n')); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		os.Remove(path)
		return errors.New("write key file failed")
	}
	return nil
}

// LoadFile reads all key versions. Missing, malformed or loosely permissioned
// files fail closed; the raw key material is never returned in an error.
func LoadFile(path string) (*Keyring, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("key file unavailable")
	}
	if !info.Mode().IsRegular() || info.Size() > maxKeyFileBytes {
		return nil, errors.New("key file must be a small regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("key file permissions must be owner-only")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("key file unreadable")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var file keyFile
	if err := decoder.Decode(&file); err != nil {
		return nil, errors.New("key file format invalid")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("key file contains trailing data")
	}
	if !keyIDPattern.MatchString(file.Active) || len(file.Keys) == 0 || len(file.Keys) > 8 {
		return nil, errors.New("key file configuration invalid")
	}
	keys := make(map[string][32]byte, len(file.Keys))
	for id, encoded := range file.Keys {
		if !keyIDPattern.MatchString(id) {
			return nil, errors.New("key file identifier invalid")
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(decoded) != 32 {
			return nil, errors.New("key file key material invalid")
		}
		var key [32]byte
		copy(key[:], decoded)
		keys[id] = key
	}
	if _, ok := keys[file.Active]; !ok {
		return nil, errors.New("active key is missing")
	}
	return &Keyring{active: file.Active, keys: keys}, nil
}

func (k *Keyring) Seal(aad, plaintext []byte) (Envelope, error) {
	if k == nil {
		return Envelope{}, errors.New("encryption keys unavailable")
	}
	key := k.keys[k.active]
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return Envelope{}, errors.New("encryption initialization failed")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Envelope{}, errors.New("encryption initialization failed")
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return Envelope{}, errors.New("encryption nonce unavailable")
	}
	return Envelope{Version: 1, KeyID: k.active, Nonce: nonce,
		Ciphertext: aead.Seal(nil, nonce, plaintext, aad)}, nil
}

func (k *Keyring) Open(aad []byte, envelope Envelope) ([]byte, error) {
	if k == nil || envelope.Version != 1 {
		return nil, errors.New("encrypted payload version unavailable")
	}
	key, ok := k.keys[envelope.KeyID]
	if !ok {
		return nil, errors.New("encrypted payload key unavailable")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, errors.New("decryption initialization failed")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(envelope.Nonce) != aead.NonceSize() {
		return nil, errors.New("encrypted payload nonce invalid")
	}
	plaintext, err := aead.Open(nil, envelope.Nonce, envelope.Ciphertext, aad)
	if err != nil {
		return nil, errors.New("encrypted payload authentication failed")
	}
	return plaintext, nil
}

// CSRFToken derives a session-bound token using a domain-separated key.
func (k *Keyring) CSRFToken(sessionToken string) (string, error) {
	if k == nil {
		return "", errors.New("CSRF key unavailable")
	}
	key := k.keys[k.active]
	derive := hmac.New(sha256.New, key[:])
	derive.Write([]byte("relay/csrf-key/v1"))
	csrfKey := derive.Sum(nil)
	mac := hmac.New(sha256.New, csrfKey)
	mac.Write([]byte(sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
