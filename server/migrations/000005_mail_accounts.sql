-- Encrypted mail account registry. Credentials are AEAD envelopes stored as
-- keyring Envelope JSON; plaintext credentials are never persisted or logged.
-- AAD domain: relay/mail_account/v1 binds owner_id, account_id and payload kind.
CREATE TABLE mail_accounts (
    account_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    address TEXT NOT NULL CHECK (length(address) BETWEEN 3 AND 254),
    imap_host TEXT NOT NULL CHECK (length(imap_host) BETWEEN 1 AND 253),
    imap_port INTEGER NOT NULL CHECK (imap_port BETWEEN 1 AND 65535),
    smtp_host TEXT NOT NULL CHECK (length(smtp_host) BETWEEN 1 AND 253),
    smtp_port INTEGER NOT NULL CHECK (smtp_port BETWEEN 1 AND 65535),
    tls_mode TEXT NOT NULL CHECK (tls_mode IN ('starttls', 'implicit_tls')),
    username TEXT NOT NULL CHECK (length(username) BETWEEN 1 AND 254),
    credential_ciphertext BLOB NOT NULL CHECK (length(credential_ciphertext) BETWEEN 64 AND 65536),
    connection_state TEXT NOT NULL CHECK (connection_state IN ('untested', 'ok', 'failed')),
    last_sync_ok_at_ms INTEGER,
    revoked_at_ms INTEGER,
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0)
);
CREATE INDEX mail_accounts_owner ON mail_accounts(owner_id, created_at_ms, account_id);
