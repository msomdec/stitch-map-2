ALTER TABLE patterns ADD COLUMN locked BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE pattern_stitches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id INTEGER NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    abbreviation TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'basic',
    library_stitch_id INTEGER,
    UNIQUE(pattern_id, abbreviation)
);
CREATE INDEX idx_pattern_stitches_pattern ON pattern_stitches(pattern_id);

-- Populate from existing data
INSERT INTO pattern_stitches (pattern_id, abbreviation, name, description, category, library_stitch_id)
SELECT DISTINCT ig.pattern_id, s.abbreviation, s.name, s.description, s.category, s.id
FROM stitch_entries se
JOIN instruction_groups ig ON se.instruction_group_id = ig.id
JOIN stitches s ON se.stitch_id = s.id;

-- Recreate stitch_entries with pattern_stitch_id FK
CREATE TABLE stitch_entries_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instruction_group_id INTEGER NOT NULL REFERENCES instruction_groups(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL,
    pattern_stitch_id INTEGER NOT NULL REFERENCES pattern_stitches(id),
    count INTEGER NOT NULL DEFAULT 1,
    into_stitch TEXT NOT NULL DEFAULT '',
    repeat_count INTEGER NOT NULL DEFAULT 1,
    UNIQUE(instruction_group_id, sort_order)
);

INSERT INTO stitch_entries_new (id, instruction_group_id, sort_order, pattern_stitch_id, count, into_stitch, repeat_count)
SELECT se.id, se.instruction_group_id, se.sort_order, ps.id, se.count, se.into_stitch, se.repeat_count
FROM stitch_entries se
JOIN instruction_groups ig ON se.instruction_group_id = ig.id
JOIN stitches s ON se.stitch_id = s.id
JOIN pattern_stitches ps ON ps.pattern_id = ig.pattern_id AND ps.abbreviation = s.abbreviation;

DROP TABLE stitch_entries;
ALTER TABLE stitch_entries_new RENAME TO stitch_entries;
CREATE INDEX idx_stitch_entries_group ON stitch_entries(instruction_group_id);
