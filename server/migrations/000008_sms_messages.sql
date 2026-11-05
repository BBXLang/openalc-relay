-- Encrypted SMS records. Sender, recipient and body are AEAD envelopes; the
-- device uploads with a stable remote event ID so retries deduplicate.
-- AAD domain: relay/sms_message/v1 binds owner_id, device_id, sms_id and the
-- payload kind (sender/recipient/body).
CREATE TABLE sms_messages (
    sms_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
    direction TEXT NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    remote_event_id TEXT NOT NULL CHECK (length(remote_event_id) BETWEEN 1 AND 128),
    sender_ciphertext BLOB NOT NULL CHECK (length(sender_ciphertext) > 0),
    recipient_ciphertext BLOB NOT NULL CHECK (length(recipient_ciphertext) > 0),
    body_ciphertext BLOB NOT NULL CHECK (length(body_ciphertext) > 0),
    device_at_ms INTEGER NOT NULL CHECK (device_at_ms > 0),
    received_at_ms INTEGER NOT NULL CHECK (received_at_ms > 0),
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0),
    UNIQUE (device_id, remote_event_id)
);
CREATE INDEX sms_messages_cursor ON sms_messages(owner_id, received_at_ms, sms_id);
