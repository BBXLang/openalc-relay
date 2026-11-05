-- Synthetic-only device exchange. No SMS bodies, numbers or command payloads.
CREATE TABLE device_pairings (
    pairing_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    created_at_ms INTEGER NOT NULL,
    expires_at_ms INTEGER NOT NULL CHECK (expires_at_ms > created_at_ms),
    used_at_ms INTEGER
);
CREATE TABLE pairing_requests (
    request_id TEXT PRIMARY KEY NOT NULL,
    pairing_id TEXT NOT NULL UNIQUE REFERENCES device_pairings(pairing_id) ON DELETE CASCADE,
    claim_hash BLOB NOT NULL CHECK (length(claim_hash) = 32),
    comparison_code TEXT NOT NULL CHECK (length(comparison_code) = 6),
    device_label TEXT NOT NULL CHECK (length(device_label) BETWEEN 1 AND 80),
    failed_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 3),
    approved_at_ms INTEGER,
    exchanged_at_ms INTEGER
);
CREATE TABLE devices (
    device_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    device_label TEXT NOT NULL,
    credential_hash BLOB NOT NULL UNIQUE CHECK (length(credential_hash) = 32),
    created_at_ms INTEGER NOT NULL,
    revoked_at_ms INTEGER
);
CREATE INDEX devices_owner ON devices(owner_id, device_id);
CREATE TABLE synthetic_events (
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
    client_event_id TEXT NOT NULL,
    received_at_ms INTEGER NOT NULL,
    PRIMARY KEY (device_id, client_event_id)
);
CREATE INDEX synthetic_events_owner ON synthetic_events(owner_id, received_at_ms, client_event_id);
CREATE TABLE synthetic_pings (
    ping_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
    created_at_ms INTEGER NOT NULL,
    acknowledged_at_ms INTEGER
);
CREATE INDEX synthetic_pings_pending ON synthetic_pings(device_id, acknowledged_at_ms, created_at_ms);
