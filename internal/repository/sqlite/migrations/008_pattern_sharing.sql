-- Share links table
CREATE TABLE IF NOT EXISTS pattern_shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    share_type TEXT NOT NULL DEFAULT 'global',
    recipient_email TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pattern_shares_token ON pattern_shares(token);
CREATE INDEX IF NOT EXISTS idx_pattern_shares_pattern ON pattern_shares(pattern_id);
CREATE INDEX IF NOT EXISTS idx_pattern_shares_email ON pattern_shares(recipient_email) WHERE recipient_email != '';

-- Origin tracking on patterns (for "Shared with Me" section)
ALTER TABLE patterns ADD COLUMN shared_from_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE patterns ADD COLUMN shared_from_name TEXT NOT NULL DEFAULT '';
