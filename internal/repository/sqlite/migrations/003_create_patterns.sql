CREATE TABLE IF NOT EXISTS patterns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    pattern_type TEXT NOT NULL DEFAULT 'round',
    hook_size TEXT NOT NULL DEFAULT '',
    yarn_weight TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_patterns_user ON patterns(user_id);

CREATE TABLE IF NOT EXISTS instruction_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL,
    label TEXT NOT NULL,
    repeat_count INTEGER NOT NULL DEFAULT 1,
    expected_count INTEGER,
    UNIQUE(pattern_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_instruction_groups_pattern ON instruction_groups(pattern_id);

CREATE TABLE IF NOT EXISTS stitch_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instruction_group_id INTEGER NOT NULL REFERENCES instruction_groups(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL,
    stitch_id INTEGER NOT NULL REFERENCES stitches(id),
    count INTEGER NOT NULL DEFAULT 1,
    into_stitch TEXT NOT NULL DEFAULT '',
    repeat_count INTEGER NOT NULL DEFAULT 1,
    notes TEXT NOT NULL DEFAULT '',
    UNIQUE(instruction_group_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_stitch_entries_group ON stitch_entries(instruction_group_id);
