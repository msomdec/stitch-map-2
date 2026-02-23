-- 006_create_pattern_images.sql
-- Image metadata and blob storage for pattern part images.

CREATE TABLE IF NOT EXISTS pattern_images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instruction_group_id INTEGER NOT NULL REFERENCES instruction_groups(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size INTEGER NOT NULL,
    storage_key TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pattern_images_group ON pattern_images(instruction_group_id);

CREATE TABLE IF NOT EXISTS file_blobs (
    storage_key TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
