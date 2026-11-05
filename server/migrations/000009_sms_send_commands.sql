-- Single-send commands with an explicit state machine: queued, claimed,
-- system_sent, delivery_receipt, failed, unknown. Once a command may have
-- reached the device send call it can never be re-queued automatically.
-- AAD domain: relay/sms_command/v1 binds owner_id, device_id, command_id and
-- the payload kind (recipient/body).
CREATE TABLE sms_send_commands (
    command_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
    recipient_ciphertext BLOB NOT NULL CHECK (length(recipient_ciphertext) > 0),
    body_ciphertext BLOB NOT NULL CHECK (length(body_ciphertext) > 0),
    status TEXT NOT NULL CHECK (status IN ('queued', 'claimed', 'system_sent', 'delivery_receipt', 'failed', 'unknown')),
    claimed_at_ms INTEGER,
    system_sent_at_ms INTEGER,
    resolved_at_ms INTEGER,
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0),
    UNIQUE (owner_id, idempotency_key)
);
CREATE INDEX sms_send_commands_device ON sms_send_commands(device_id, status);
CREATE INDEX sms_send_commands_owner ON sms_send_commands(owner_id, created_at_ms, command_id);
