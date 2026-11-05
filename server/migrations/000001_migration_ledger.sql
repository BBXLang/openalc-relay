-- Infrastructure only: business tables require authentication and payload encryption first.
CREATE TABLE schema_migrations (
    version TEXT PRIMARY KEY NOT NULL,
    checksum TEXT NOT NULL CHECK (length(checksum) = 64),
    applied_at TEXT NOT NULL
);
