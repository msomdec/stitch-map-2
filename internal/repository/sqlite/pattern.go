package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
		`INSERT INTO patterns (user_id, name, description, pattern_type, hook_size, yarn_weight, difficulty, locked, shared_from_user_id, shared_from_name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pattern.UserID, pattern.Name, pattern.Description, pattern.PatternType,
		pattern.HookSize, pattern.YarnWeight, pattern.Difficulty, pattern.Locked,
		pattern.SharedFromUserID, pattern.SharedFromName, now, now,
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
		`SELECT id, user_id, name, description, pattern_type, hook_size, yarn_weight, difficulty, locked, shared_from_user_id, shared_from_name, created_at, updated_at
		 FROM patterns WHERE id = ?`, id,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.PatternType,
		&p.HookSize, &p.YarnWeight, &p.Difficulty, &p.Locked, &p.SharedFromUserID, &p.SharedFromName, &p.CreatedAt, &p.UpdatedAt)
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

func (r *patternRepo) GetNamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT id, name FROM patterns WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get names by ids: %w", err)
	}
	defer rows.Close()

	names := make(map[int64]string, len(ids))
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan pattern name: %w", err)
		}
		names[id] = name
	}
	return names, rows.Err()
}

func (r *patternRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Pattern, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, description, pattern_type, hook_size, yarn_weight, difficulty, locked, shared_from_user_id, shared_from_name, created_at, updated_at
		 FROM patterns WHERE user_id = ? AND shared_from_user_id IS NULL ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	defer rows.Close()

	var patterns []domain.Pattern
	for rows.Next() {
		var p domain.Pattern
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.PatternType,
			&p.HookSize, &p.YarnWeight, &p.Difficulty, &p.Locked, &p.SharedFromUserID, &p.SharedFromName, &p.CreatedAt, &p.UpdatedAt); err != nil {
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

func (r *patternRepo) ListSharedWithUser(ctx context.Context, userID int64) ([]domain.Pattern, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, description, pattern_type, hook_size, yarn_weight, difficulty, locked, shared_from_user_id, shared_from_name, created_at, updated_at
		 FROM patterns WHERE user_id = ? AND shared_from_user_id IS NOT NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list shared patterns: %w", err)
	}
	defer rows.Close()

	var patterns []domain.Pattern
	for rows.Next() {
		var p domain.Pattern
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.PatternType,
			&p.HookSize, &p.YarnWeight, &p.Difficulty, &p.Locked, &p.SharedFromUserID, &p.SharedFromName, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan shared pattern: %w", err)
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

const patternSummaryQuery = `
SELECT p.id, p.user_id, p.name, p.description, p.pattern_type, p.hook_size, p.yarn_weight,
       p.difficulty, p.locked, p.shared_from_user_id, p.shared_from_name,
       p.created_at, p.updated_at,
       COUNT(DISTINCT ig.id) as group_count,
       COALESCE(SUM(se.count * se.repeat_count * ig.repeat_count), 0) as stitch_count
FROM patterns p
LEFT JOIN instruction_groups ig ON ig.pattern_id = p.id
LEFT JOIN stitch_entries se ON se.instruction_group_id = ig.id
`

func scanPatternSummary(rows *sql.Rows) (domain.PatternSummary, error) {
	var s domain.PatternSummary
	err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Description, &s.PatternType,
		&s.HookSize, &s.YarnWeight, &s.Difficulty, &s.Locked,
		&s.SharedFromUserID, &s.SharedFromName, &s.CreatedAt, &s.UpdatedAt,
		&s.GroupCount, &s.StitchCount)
	return s, err
}

func (r *patternRepo) ListSummaryByUser(ctx context.Context, userID int64) ([]domain.PatternSummary, error) {
	query := patternSummaryQuery + `WHERE p.user_id = ? AND p.shared_from_user_id IS NULL
GROUP BY p.id
ORDER BY p.updated_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list pattern summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.PatternSummary
	for rows.Next() {
		s, err := scanPatternSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pattern summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (r *patternRepo) ListSummarySharedWithUser(ctx context.Context, userID int64) ([]domain.PatternSummary, error) {
	query := patternSummaryQuery + `WHERE p.user_id = ? AND p.shared_from_user_id IS NOT NULL
GROUP BY p.id
ORDER BY p.created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list shared pattern summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.PatternSummary
	for rows.Next() {
		s, err := scanPatternSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan shared pattern summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (r *patternRepo) SearchSummaryByUser(ctx context.Context, userID int64, filter domain.PatternFilter) ([]domain.PatternSummary, error) {
	query := patternSummaryQuery + `WHERE p.user_id = ? AND p.shared_from_user_id IS NULL`
	args := []interface{}{userID}

	if filter.Query != "" {
		query += ` AND (p.name LIKE ? OR p.description LIKE ?)`
		like := "%" + filter.Query + "%"
		args = append(args, like, like)
	}
	if filter.Type != "" {
		query += ` AND p.pattern_type = ?`
		args = append(args, filter.Type)
	}
	if filter.Difficulty != "" {
		query += ` AND p.difficulty = ?`
		args = append(args, filter.Difficulty)
	}

	query += ` GROUP BY p.id`

	switch filter.Sort {
	case "name":
		query += ` ORDER BY p.name ASC`
	case "created":
		query += ` ORDER BY p.created_at DESC`
	case "stitches":
		query += ` ORDER BY stitch_count DESC`
	default:
		query += ` ORDER BY p.updated_at DESC`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search pattern summaries: %w", err)
	}
	defer rows.Close()

	var summaries []domain.PatternSummary
	for rows.Next() {
		s, err := scanPatternSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pattern summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
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

func (r *patternRepo) DuplicateAsShared(ctx context.Context, id int64, newUserID int64, sharedFromUserID int64, sharedFromName string) (*domain.Pattern, error) {
	original, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get original: %w", err)
	}

	dup := &domain.Pattern{
		UserID:            newUserID,
		Name:              original.Name,
		Description:       original.Description,
		PatternType:       original.PatternType,
		HookSize:          original.HookSize,
		YarnWeight:        original.YarnWeight,
		Difficulty:        original.Difficulty,
		Locked:            true,
		SharedFromUserID:  &sharedFromUserID,
		SharedFromName:    sharedFromName,
		PatternStitches:   original.PatternStitches,
		InstructionGroups: original.InstructionGroups,
	}

	if err := r.Create(ctx, dup); err != nil {
		return nil, fmt.Errorf("create shared duplicate: %w", err)
	}

	// Copy images from the original pattern to the duplicate.
	if err := r.copyImages(ctx, original, dup); err != nil {
		return nil, fmt.Errorf("copy images: %w", err)
	}

	return dup, nil
}

// copyImages duplicates all pattern_images and their file_blobs from the
// original pattern to the duplicate, mapping groups by sort_order.
func (r *patternRepo) copyImages(ctx context.Context, original, dup *domain.Pattern) error {
	// Build sort_order -> new group ID mapping from the duplicate.
	dupGroupBySort := make(map[int]int64, len(dup.InstructionGroups))
	for _, g := range dup.InstructionGroups {
		dupGroupBySort[g.SortOrder] = g.ID
	}

	for _, g := range original.InstructionGroups {
		newGroupID, ok := dupGroupBySort[g.SortOrder]
		if !ok {
			continue
		}

		rows, err := r.db.QueryContext(ctx,
			`SELECT pi.filename, pi.content_type, pi.size, pi.storage_key, pi.sort_order, pi.created_at
			 FROM pattern_images pi WHERE pi.instruction_group_id = ? ORDER BY pi.sort_order`, g.ID)
		if err != nil {
			return fmt.Errorf("load images for group %d: %w", g.ID, err)
		}

		type imgRow struct {
			Filename    string
			ContentType string
			Size        int64
			StorageKey  string
			SortOrder   int
			CreatedAt   time.Time
		}
		var imgs []imgRow
		for rows.Next() {
			var img imgRow
			if err := rows.Scan(&img.Filename, &img.ContentType, &img.Size, &img.StorageKey, &img.SortOrder, &img.CreatedAt); err != nil {
				rows.Close()
				return fmt.Errorf("scan image: %w", err)
			}
			imgs = append(imgs, img)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, img := range imgs {
			// Copy the file blob with a new storage key.
			newKey := img.StorageKey + "-copy-" + fmt.Sprintf("%d", dup.ID)
			_, err := r.db.ExecContext(ctx,
				`INSERT INTO file_blobs (storage_key, data)
				 SELECT ?, data FROM file_blobs WHERE storage_key = ?`,
				newKey, img.StorageKey)
			if err != nil {
				return fmt.Errorf("copy file blob: %w", err)
			}

			_, err = r.db.ExecContext(ctx,
				`INSERT INTO pattern_images (instruction_group_id, filename, content_type, size, storage_key, sort_order, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				newGroupID, img.Filename, img.ContentType, img.Size, newKey, img.SortOrder, img.CreatedAt)
			if err != nil {
				return fmt.Errorf("insert copied image: %w", err)
			}
		}
	}

	return nil
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
