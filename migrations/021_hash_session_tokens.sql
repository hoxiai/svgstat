ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS token_hash TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_token_hash
ON sessions(token_hash)
WHERE token_hash IS NOT NULL;

COMMENT ON COLUMN sessions.token_hash IS 'SHA-256 hash used for session lookup; legacy plaintext sessions remain valid until expiry';
