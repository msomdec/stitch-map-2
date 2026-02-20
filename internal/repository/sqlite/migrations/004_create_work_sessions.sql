CREATE TABLE IF NOT EXISTS work_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    current_group_index INTEGER NOT NULL DEFAULT 0,
    current_group_repeat INTEGER NOT NULL DEFAULT 0,
    current_stitch_index INTEGER NOT NULL DEFAULT 0,
    current_stitch_repeat INTEGER NOT NULL DEFAULT 0,
    current_stitch_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_work_sessions_user ON work_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_work_sessions_pattern ON work_sessions(pattern_id);
CREATE INDEX IF NOT EXISTS idx_work_sessions_status ON work_sessions(status);
