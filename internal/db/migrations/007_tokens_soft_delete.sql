-- Soft-delete column for tokens. A revoked token stays visible in the
-- user's list (so they remember the row existed); a deleted token is
-- hidden from every read path.
ALTER TABLE tokens ADD COLUMN deleted_at TIMESTAMP;

CREATE INDEX idx_tokens_deleted ON tokens(deleted_at);
