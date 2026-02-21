-- 005_pattern_editor_overhaul.sql
-- Add difficulty to patterns, notes to instruction_groups, drop notes from stitch_entries.

ALTER TABLE patterns ADD COLUMN difficulty TEXT NOT NULL DEFAULT '';
ALTER TABLE instruction_groups ADD COLUMN notes TEXT NOT NULL DEFAULT '';
ALTER TABLE stitch_entries DROP COLUMN notes;
