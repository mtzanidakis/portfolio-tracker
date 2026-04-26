-- Optional expiry on API tokens. NULL = never expires (existing rows
-- keep their current behaviour). Auth lookup filters out rows where
-- expires_at is in the past.
ALTER TABLE tokens ADD COLUMN expires_at TIMESTAMP;
