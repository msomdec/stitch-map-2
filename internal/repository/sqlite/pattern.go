package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// patternRepo implements domain.PatternRepository using SQLite.
type patternRepo struct {
	db *sql.DB
}

func (r *patternRepo) Create(ctx context.Context, pattern *domain.Pattern) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO patterns (user_id, name, description, pattern_type, hook_size, yarn_weight, difficulty, locked, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pattern.UserID, pattern.Name, pattern.Description, pattern.PatternType,
		pattern.HookSize, pattern.YarnWeight, pattern.Difficulty, pattern.Locked, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert pattern: %w", err)
	}

	patternID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get pattern id: %w", err)
	}

	psMap, err := insertPatternStitches(ctx, tx, patternID, pattern.PatternStitches)
	if err != nil {
		return err
	}

	if err := insertGroups(ctx, tx, patternID, pattern.InstructionGroups, psMap); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	pattern.ID = patternID
	pattern.CreatedAt = now
	pattern.UpdatedAt = now
	return nil
}

func (r *patternRepo) GetByID(ctx context.Context, id int64) (*domain.Pattern, error) {
	p := &domain.Pattern{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, description, pattern_type, hook_size, yarn_weight, difficulty, locked, created_at, updated_at
		 FROM patterns WHERE id = ?`, id,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.PatternType,
		&p.HookSize, &p.YarnWeight, &p.Difficulty, &p.Locked, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get pattern: %w", err)
	}

	ps, err := r.loadPatternStitches(ctx, id)
	if err != nil {
		return nil, err
	}
	p.PatternStitches = ps

	groups, err := r.loadGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	p.InstructionGroups = groups
	return p, nil
}

func (r *patternRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Pattern, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, description, pattern_type, hook_size, yarn_weight, difficulty, locked, created_at, updated_at
		 FROM patterns WHERE user_id = ? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	defer rows.Close()

	var patterns []domain.Pattern
	for rows.Next() {
		var p domain.Pattern
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.PatternType,
			&p.HookSize, &p.YarnWeight, &p.Difficulty, &p.Locked, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan pattern: %w", err)
		}
		patterns = append(patterns, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range patterns {
		ps, err := r.loadPatternStitches(ctx, patterns[i].ID)
		if err != nil {
			return nil, fmt.Errorf("load pattern stitches for pattern %d: %w", patterns[i].ID, err)
		}
		patterns[i].PatternStitches = ps

		groups, err := r.loadGroups(ctx, patterns[i].ID)
		if err != nil {
			return nil, fmt.Errorf("load groups for pattern %d: %w", patterns[i].ID, err)
		}
		patterns[i].InstructionGroups = groups
	}

	return patterns, nil
}

func (r *patternRepo) Update(ctx context.Context, pattern *domain.Pattern) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx,
		`UPDATE patterns SET name = ?, description = ?, pattern_type = ?, hook_size = ?, yarn_weight = ?, difficulty = ?, updated_at = ?
		 WHERE id = ?`,
		pattern.Name, pattern.Description, pattern.PatternType,
		pattern.HookSize, pattern.YarnWeight, pattern.Difficulty, now, pattern.ID,
	)
	if err != nil {
		return fmt.Errorf("update pattern: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	// Preserve images across group delete/re-insert.
	// Load existing images keyed by their group's sort_order before deletion.
	savedImages, err := loadImagesForPattern(ctx, tx, pattern.ID)
	if err != nil {
		return fmt.Errorf("preserve images: %w", err)
	}

	// Delete existing groups (cascades to stitch_entries and pattern_images) and pattern_stitches, then re-insert.
	if _, err := tx.ExecContext(ctx, "DELETE FROM instruction_groups WHERE pattern_id = ?", pattern.ID); err != nil {
		return fmt.Errorf("delete groups: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM pattern_stitches WHERE pattern_id = ?", pattern.ID); err != nil {
		return fmt.Errorf("delete pattern stitches: %w", err)
	}

	psMap, err := insertPatternStitches(ctx, tx, pattern.ID, pattern.PatternStitches)
	if err != nil {
		return err
	}

	if err := insertGroups(ctx, tx, pattern.ID, pattern.InstructionGroups, psMap); err != nil {
		return err
	}

	// Re-insert preserved images with new group IDs (matched by sort_order).
	if err := restoreImages(ctx, tx, pattern.InstructionGroups, savedImages); err != nil {
		return fmt.Errorf("restore images: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	pattern.UpdatedAt = now
	return nil
}

func (r *patternRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM patterns WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete pattern: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *patternRepo) Duplicate(ctx context.Context, id int64, newUserID int64) (*domain.Pattern, error) {
	original, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get original: %w", err)
	}

	dup := &domain.Pattern{
		UserID:            newUserID,
		Name:              original.Name + " (Copy)",
		Description:       original.Description,
		PatternType:       original.PatternType,
		HookSize:          original.HookSize,
		YarnWeight:        original.YarnWeight,
		Difficulty:        original.Difficulty,
		Locked:            false, // copies are always unlocked
		PatternStitches:   original.PatternStitches,
		InstructionGroups: original.InstructionGroups,
	}

	if err := r.Create(ctx, dup); err != nil {
		return nil, fmt.Errorf("create duplicate: %w", err)
	}

	return dup, nil
}

// insertPatternStitches inserts pattern_stitches rows and returns a mapping from
// old ID (or index) to new real database ID. When a PatternStitch has ID != 0
// (duplicate case), the mapping is oldID -> newID. When ID == 0 (new pattern from
// service), the mapping is the slice index -> newID.
func insertPatternStitches(ctx context.Context, tx *sql.Tx, patternID int64, stitches []domain.PatternStitch) (map[int64]int64, error) {
	psMap := make(map[int64]int64, len(stitches))

	for i := range stitches {
		ps := &stitches[i]
		result, err := tx.ExecContext(ctx,
			`INSERT INTO pattern_stitches (pattern_id, abbreviation, name, description, category, library_stitch_id)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			patternID, ps.Abbreviation, ps.Name, ps.Description, ps.Category, ps.LibraryStitchID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert pattern stitch %d: %w", i, err)
		}

		newID, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("get pattern stitch id: %w", err)
		}

		// Map old ID -> new ID for remapping stitch entries.
		if ps.ID != 0 {
			psMap[ps.ID] = newID
		} else {
			psMap[int64(i)] = newID
		}
		ps.ID = newID
		ps.PatternID = patternID
	}

	return psMap, nil
}

func insertGroups(ctx context.Context, tx *sql.Tx, patternID int64, groups []domain.InstructionGroup, psMap map[int64]int64) error {
	for i := range groups {
		g := &groups[i]
		result, err := tx.ExecContext(ctx,
			`INSERT INTO instruction_groups (pattern_id, sort_order, label, repeat_count, expected_count, notes)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			patternID, g.SortOrder, g.Label, g.RepeatCount, g.ExpectedCount, g.Notes,
		)
		if err != nil {
			return fmt.Errorf("insert group %d: %w", i, err)
		}

		groupID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get group id: %w", err)
		}
		g.ID = groupID
		g.PatternID = patternID

		for j := range g.StitchEntries {
			e := &g.StitchEntries[j]

			// Remap PatternStitchID using the psMap.
			mappedID := e.PatternStitchID
			if newID, ok := psMap[e.PatternStitchID]; ok {
				mappedID = newID
			}

			res, err := tx.ExecContext(ctx,
				`INSERT INTO stitch_entries (instruction_group_id, sort_order, pattern_stitch_id, count, into_stitch, repeat_count)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				groupID, e.SortOrder, mappedID, e.Count, e.IntoStitch, e.RepeatCount,
			)
			if err != nil {
				return fmt.Errorf("insert entry %d/%d: %w", i, j, err)
			}

			entryID, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("get entry id: %w", err)
			}
			e.ID = entryID
			e.InstructionGroupID = groupID
			e.PatternStitchID = mappedID
		}
	}
	return nil
}

func (r *patternRepo) loadPatternStitches(ctx context.Context, patternID int64) ([]domain.PatternStitch, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pattern_id, abbreviation, name, description, category, library_stitch_id
		 FROM pattern_stitches WHERE pattern_id = ? ORDER BY id`, patternID)
	if err != nil {
		return nil, fmt.Errorf("load pattern stitches: %w", err)
	}
	defer rows.Close()

	var stitches []domain.PatternStitch
	for rows.Next() {
		var ps domain.PatternStitch
		if err := rows.Scan(&ps.ID, &ps.PatternID, &ps.Abbreviation, &ps.Name,
			&ps.Description, &ps.Category, &ps.LibraryStitchID); err != nil {
			return nil, fmt.Errorf("scan pattern stitch: %w", err)
		}
		stitches = append(stitches, ps)
	}
	return stitches, rows.Err()
}

func (r *patternRepo) loadGroups(ctx context.Context, patternID int64) ([]domain.InstructionGroup, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pattern_id, sort_order, label, repeat_count, expected_count, notes
		 FROM instruction_groups WHERE pattern_id = ? ORDER BY sort_order`, patternID)
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	defer rows.Close()

	var groups []domain.InstructionGroup
	for rows.Next() {
		var g domain.InstructionGroup
		if err := rows.Scan(&g.ID, &g.PatternID, &g.SortOrder, &g.Label, &g.RepeatCount, &g.ExpectedCount, &g.Notes); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range groups {
		entries, err := r.loadEntries(ctx, groups[i].ID)
		if err != nil {
			return nil, err
		}
		groups[i].StitchEntries = entries
	}
	return groups, nil
}

func (r *patternRepo) loadEntries(ctx context.Context, groupID int64) ([]domain.StitchEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, instruction_group_id, sort_order, pattern_stitch_id, count, into_stitch, repeat_count
		 FROM stitch_entries WHERE instruction_group_id = ? ORDER BY sort_order`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.StitchEntry
	for rows.Next() {
		var e domain.StitchEntry
		if err := rows.Scan(&e.ID, &e.InstructionGroupID, &e.SortOrder, &e.PatternStitchID,
			&e.Count, &e.IntoStitch, &e.RepeatCount); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// savedImage holds a pattern_images row along with the sort_order of the
// instruction group it belonged to, so it can be reassociated after groups
// are deleted and re-inserted.
type savedImage struct {
	GroupSortOrder     int
	Filename           string
	ContentType        string
	Size               int64
	StorageKey         string
	SortOrder          int
	CreatedAt          time.Time
}

// loadImagesForPattern loads all pattern_images for a pattern's groups,
// keyed by the group's sort_order. This is called before the groups are
// deleted so the images can be restored after re-insertion.
func loadImagesForPattern(ctx context.Context, tx *sql.Tx, patternID int64) ([]savedImage, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT ig.sort_order, pi.filename, pi.content_type, pi.size, pi.storage_key, pi.sort_order, pi.created_at
		 FROM pattern_images pi
		 JOIN instruction_groups ig ON pi.instruction_group_id = ig.id
		 WHERE ig.pattern_id = ?
		 ORDER BY ig.sort_order, pi.sort_order`, patternID)
	if err != nil {
		return nil, fmt.Errorf("load images for pattern: %w", err)
	}
	defer rows.Close()

	var images []savedImage
	for rows.Next() {
		var img savedImage
		if err := rows.Scan(&img.GroupSortOrder, &img.Filename, &img.ContentType,
			&img.Size, &img.StorageKey, &img.SortOrder, &img.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan saved image: %w", err)
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

// restoreImages re-inserts preserved pattern_images rows using the new
// instruction group IDs (matched by sort_order).
func restoreImages(ctx context.Context, tx *sql.Tx, groups []domain.InstructionGroup, images []savedImage) error {
	if len(images) == 0 {
		return nil
	}

	// Build sort_order -> new group ID mapping.
	groupBySort := make(map[int]int64, len(groups))
	for i := range groups {
		groupBySort[groups[i].SortOrder] = groups[i].ID
	}

	for _, img := range images {
		newGroupID, ok := groupBySort[img.GroupSortOrder]
		if !ok {
			// The group was removed in this edit; the image's blob is orphaned.
			// Clean it up.
			tx.ExecContext(ctx, "DELETE FROM file_blobs WHERE storage_key = ?", img.StorageKey)
			continue
		}

		_, err := tx.ExecContext(ctx,
			`INSERT INTO pattern_images (instruction_group_id, filename, content_type, size, storage_key, sort_order, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newGroupID, img.Filename, img.ContentType, img.Size, img.StorageKey, img.SortOrder, img.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("restore image %q: %w", img.StorageKey, err)
		}
	}

	return nil
}
