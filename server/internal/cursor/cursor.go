// Package cursor provides the stable opaque keyset cursors shared by the list
// endpoints. A cursor pins the last returned (received_at_ms, id) position so
// records inserted afterwards cannot shift an in-flight page.
package cursor

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

// ErrInvalid reports a malformed or foreign cursor value.
var ErrInvalid = errors.New("cursor invalid")

type position struct {
	AtMs int64  `json:"at_ms"`
	ID   string `json:"id"`
}

// Encode packs a keyset position into an opaque cursor value.
func Encode(atMs int64, id string) (string, error) {
	if id == "" {
		return "", ErrInvalid
	}
	data, err := json.Marshal(position{AtMs: atMs, ID: id})
	if err != nil {
		return "", ErrInvalid
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// Decode unpacks a cursor. An empty value starts a first page and returns no
// position; a malformed value fails with ErrInvalid instead of a best guess.
func Decode(value string) (atMs int64, id string, err error) {
	if value == "" {
		return 0, "", nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, "", ErrInvalid
	}
	var pos position
	if err := json.Unmarshal(data, &pos); err != nil || pos.ID == "" || pos.AtMs <= 0 {
		return 0, "", ErrInvalid
	}
	return pos.AtMs, pos.ID, nil
}
