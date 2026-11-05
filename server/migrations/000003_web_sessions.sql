CREATE TABLE web_sessions (
    token_hash BLOB PRIMARY KEY NOT NULL CHECK (length(token_hash) = 32),
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0),
    expires_at_ms INTEGER NOT NULL CHECK (expires_at_ms > created_at_ms),
    revoked_at_ms INTEGER
);
CREATE INDEX web_sessions_owner ON web_sessions(owner_id, expires_at_ms);
