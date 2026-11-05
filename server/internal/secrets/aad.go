package secrets

import (
	"encoding/json"
	"errors"
	"strings"
)

// AAD domains name the stored ciphertext kinds. Each domain is embedded as the
// first component of the additional authenticated data so an envelope sealed
// for one kind of payload can never be opened as another kind.
const (
	DomainMailAccount    = "mail_account"
	DomainMailMessage    = "mail_message"
	DomainMailAttachment = "mail_attachment"
	DomainSMSMessage     = "sms_message"
	DomainSMSCommand     = "sms_command"
)

// AAD binds an envelope to its storage coordinates:
//
//	"relay/<domain>/v1\x00<owner>\x00<scope>\x00<record>\x00<field>"
//
// scope is the owning account or device, record is the row identifier and
// field names the payload inside the row. Changing any coordinate makes Open
// fail, so ciphertexts cannot be replayed across owners, devices, records or
// payload kinds. Every component must be nonempty and free of NUL bytes.
func AAD(domain, ownerID, scopeID, recordID, field string) ([]byte, error) {
	for _, part := range []string{domain, ownerID, scopeID, recordID, field} {
		if part == "" || strings.Contains(part, "\x00") {
			return nil, errors.New("AAD component must be nonempty and NUL-free")
		}
	}
	return []byte("relay/" + domain + "/v1\x00" + ownerID + "\x00" + scopeID + "\x00" +
		recordID + "\x00" + field), nil
}

// SealToBlob seals plaintext into a JSON envelope suitable for a BLOB column.
func (k *Keyring) SealToBlob(aad, plaintext []byte) ([]byte, error) {
	envelope, err := k.Seal(aad, plaintext)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, errors.New("envelope serialization failed")
	}
	return data, nil
}

// OpenFromBlob parses a stored JSON envelope and opens it under the same AAD.
func (k *Keyring) OpenFromBlob(aad []byte, stored []byte) ([]byte, error) {
	var envelope Envelope
	if err := json.Unmarshal(stored, &envelope); err != nil {
		return nil, errors.New("stored envelope format invalid")
	}
	return k.Open(aad, envelope)
}
