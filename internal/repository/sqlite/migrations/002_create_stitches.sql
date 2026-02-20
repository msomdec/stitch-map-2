-- 002_create_stitches.sql
CREATE TABLE IF NOT EXISTS stitches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    abbreviation TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'basic',
    is_custom BOOLEAN NOT NULL DEFAULT FALSE,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(abbreviation, user_id)
);

CREATE INDEX IF NOT EXISTS idx_stitches_user ON stitches(user_id);
CREATE INDEX IF NOT EXISTS idx_stitches_category ON stitches(category);
