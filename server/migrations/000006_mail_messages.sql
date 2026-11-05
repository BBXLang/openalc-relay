-- Encrypted mail messages. Subject, headers and body are separate AEAD
-- envelopes; sanitized display is decided by body_format at render time.
-- AAD domain: relay/mail_message/v1 binds owner_id, account_id, message_id
-- and the payload kind (subject/headers/body).
CREATE TABLE mail_messages (
    message_id TEXT PRIMARY KEY NOT NULL,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES mail_accounts(account_id) ON DELETE CASCADE,
    folder TEXT NOT NULL CHECK (length(folder) BETWEEN 1 AND 256),
    remote_message_id TEXT NOT NULL CHECK (length(remote_message_id) BETWEEN 1 AND 998),
    received_at_ms INTEGER NOT NULL CHECK (received_at_ms > 0),
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    is_unread INTEGER NOT NULL CHECK (is_unread IN (0, 1)),
    has_attachments INTEGER NOT NULL CHECK (has_attachments IN (0, 1)),
    body_format TEXT NOT NULL CHECK (body_format IN ('text_plain', 'sanitized_html')),
    subject_ciphertext BLOB NOT NULL CHECK (length(subject_ciphertext) > 0),
    headers_ciphertext BLOB NOT NULL CHECK (length(headers_ciphertext) > 0),
    body_ciphertext BLOB NOT NULL CHECK (length(body_ciphertext) > 0),
    attachment_count INTEGER NOT NULL CHECK (attachment_count BETWEEN 0 AND 1000),
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0),
    UNIQUE (account_id, folder, remote_message_id)
);
CREATE INDEX mail_messages_cursor ON mail_messages(owner_id, received_at_ms, message_id);
