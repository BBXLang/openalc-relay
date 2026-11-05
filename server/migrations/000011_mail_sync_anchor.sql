-- Inbox sync anchors for mail accounts (FORMAL-UI-MAIL-PLAN batch 3 minimal
-- sync slice). The anchor is the last processed IMAP state for INBOX:
-- UIDVALIDITY plus the highest UID the sync has processed (imported or
-- deliberately skipped). NULL columns mean "never synced"; a UIDVALIDITY
-- change on the server restarts the incremental pull. Existing rows keep
-- NULL and the next sync behaves like the first one (bounded full pull).
ALTER TABLE mail_accounts ADD COLUMN sync_uidvalidity INTEGER;
ALTER TABLE mail_accounts ADD COLUMN sync_highest_uid INTEGER;
