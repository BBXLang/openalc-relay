-- Controlled metadata audit trail written in the same transaction as the
-- state change it records. Entries never contain credentials, message bodies
-- or phone numbers; detail holds bounded JSON metadata only.
CREATE TABLE audit_entries (
    entry_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('owner_session', 'device', 'system')),
    actor_id TEXT NOT NULL CHECK (length(actor_id) BETWEEN 1 AND 128),
    action TEXT NOT NULL CHECK (length(action) BETWEEN 1 AND 64),
    subject_kind TEXT NOT NULL CHECK (length(subject_kind) BETWEEN 1 AND 32),
    subject_id TEXT NOT NULL CHECK (length(subject_id) BETWEEN 1 AND 128),
    detail TEXT CHECK (detail IS NULL OR length(detail) BETWEEN 1 AND 2048),
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0)
);
CREATE INDEX audit_entries_owner ON audit_entries(owner_id, created_at_ms, entry_id);
