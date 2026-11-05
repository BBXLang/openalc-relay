-- This separate test database must never receive SMS bodies, telephone numbers, or credentials.
CREATE TABLE schema_migrations (
    version TEXT PRIMARY KEY NOT NULL,
    checksum TEXT NOT NULL CHECK (length(checksum) = 64),
    applied_at TEXT NOT NULL
);

CREATE TABLE synthetic_events (
    installation_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    fixture_id TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('injected', 'sms_broadcast')),
    received_at_ms INTEGER NOT NULL,
    subscription_id INTEGER,
    payload_checksum TEXT NOT NULL,
    PRIMARY KEY (installation_id, event_id)
);
