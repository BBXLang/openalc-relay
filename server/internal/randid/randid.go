// Package randid generates random server-side record identifiers.
package randid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// New returns a random UUIDv4-format identifier for record primary keys.
func New() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("random identifier unavailable")
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:], nil
}
