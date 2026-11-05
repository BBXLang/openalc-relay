-- Attachment metadata plus optional on-demand encrypted content. Content is
-- only fetched and stored after the user explicitly requests it; the per
-- account size limit is enforced by the application layer.
-- AAD domain: relay/mail_attachment/v1 binds owner_id, account_id,
-- attachment_id and the payload kind (content).
CREATE TABLE mail_attachments (
    attachment_id TEXT PRIMARY KEY NOT NULL,
    message_id TEXT NOT NULL REFERENCES mail_messages(message_id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL REFERENCES owners(owner_id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES mail_accounts(account_id) ON DELETE CASCADE,
    filename TEXT NOT NULL CHECK (length(filename) BETWEEN 1 AND 255),
    mime_type TEXT NOT NULL CHECK (length(mime_type) BETWEEN 1 AND 127),
    size_bytes INTEGER NOT NULL CHECK (size_bytes BETWEEN 0 AND 67108864),
    content_ciphertext BLOB CHECK (content_ciphertext IS NULL OR length(content_ciphertext) > 0),
    download_state TEXT NOT NULL CHECK (download_state IN ('remote_only', 'stored')),
    created_at_ms INTEGER NOT NULL CHECK (created_at_ms > 0),
    CHECK ((download_state = 'stored') = (content_ciphertext IS NOT NULL))
);
CREATE INDEX mail_attachments_message ON mail_attachments(message_id, attachment_id);
CREATE INDEX mail_attachments_owner ON mail_attachments(owner_id, account_id, attachment_id);
