-- One local owner for the first self-hosted release. No remote registration path.
CREATE TABLE owners (
    owner_id TEXT PRIMARY KEY NOT NULL,
    singleton INTEGER NOT NULL UNIQUE CHECK (singleton = 1),
    password_salt BLOB NOT NULL CHECK (length(password_salt) = 16),
    password_hash BLOB NOT NULL CHECK (length(password_hash) = 32),
    argon_time INTEGER NOT NULL CHECK (argon_time BETWEEN 1 AND 10),
    argon_memory_kib INTEGER NOT NULL CHECK (argon_memory_kib BETWEEN 8192 AND 262144),
    argon_threads INTEGER NOT NULL CHECK (argon_threads BETWEEN 1 AND 8),
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0)
);
