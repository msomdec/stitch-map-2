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
		`INSERT INTO patterns (user_id, name, description, pattern_type, hook_size, yarn_weight, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pattern.UserID, pattern.Name, pattern.Description, pattern.PatternType,
		pattern.HookSize, pattern.YarnWeight, pattern.Notes, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert pattern: %w", err)
	}

	patternID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get pattern id: %w", err)
	}

	if err := insertGroups(ctx, tx, patternID, pattern.InstructionGroups); err != nil {
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
		`SELECT id, user_id, name, description, pattern_type, hook_size, yarn_weight, notes, created_at, updated_at
		 FROM patterns WHERE id = ?`, id,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.PatternType,
		&p.HookSize, &p.YarnWeight, &p.Notes, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get pattern: %w", err)
	}

	groups, err := r.loadGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	p.InstructionGroups = groups
	return p, nil
}

func (r *patternRepo) ListByUser(ctx context.Context, userID int64) ([]domain.Pattern, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, description, pattern_type, hook_size, yarn_weight, notes, created_at, updated_at
		 FROM patterns WHERE user_id = ? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	defer rows.Close()

	var patterns []domain.Pattern
	for rows.Next() {
		var p domain.Pattern
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.PatternType,
			&p.HookSize, &p.YarnWeight, &p.Notes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan pattern: %w", err)
		}
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

func (r *patternRepo) Update(ctx context.Context, pattern *domain.Pattern) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx,
		`UPDATE patterns SET name = ?, description = ?, pattern_type = ?, hook_size = ?, yarn_weight = ?, notes = ?, updated_at = ?
		 WHERE id = ?`,
		pattern.Name, pattern.Description, pattern.PatternType,
		pattern.HookSize, pattern.YarnWeight, pattern.Notes, now, pattern.ID,
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

	// Delete existing groups and stitch entries (cascade), then re-insert.
	if _, err := tx.ExecContext(ctx, "DELETE FROM instruction_groups WHERE pattern_id = ?", pattern.ID); err != nil {
		return fmt.Errorf("delete groups: %w", err)
	}

	if err := insertGroups(ctx, tx, pattern.ID, pattern.InstructionGroups); err != nil {
		return err
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
		Notes:             original.Notes,
		InstructionGroups: original.InstructionGroups,
	}

	if err := r.Create(ctx, dup); err != nil {
		return nil, fmt.Errorf("create duplicate: %w", err)
	}

	return dup, nil
}

func insertGroups(ctx context.Context, tx *sql.Tx, patternID int64, groups []domain.InstructionGroup) error {
	for i := range groups {
		g := &groups[i]
		result, err := tx.ExecContext(ctx,
			`INSERT INTO instruction_groups (pattern_id, sort_order, label, repeat_count, expected_count)
			 VALUES (?, ?, ?, ?, ?)`,
			patternID, g.SortOrder, g.Label, g.RepeatCount, g.ExpectedCount,
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
			res, err := tx.ExecContext(ctx,
				`INSERT INTO stitch_entries (instruction_group_id, sort_order, stitch_id, count, into_stitch, repeat_count, notes)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				groupID, e.SortOrder, e.StitchID, e.Count, e.IntoStitch, e.RepeatCount, e.Notes,
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
		}
	}
	return nil
}

func (r *patternRepo) loadGroups(ctx context.Context, patternID int64) ([]domain.InstructionGroup, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pattern_id, sort_order, label, repeat_count, expected_count
		 FROM instruction_groups WHERE pattern_id = ? ORDER BY sort_order`, patternID)
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	defer rows.Close()

	var groups []domain.InstructionGroup
	for rows.Next() {
		var g domain.InstructionGroup
		if err := rows.Scan(&g.ID, &g.PatternID, &g.SortOrder, &g.Label, &g.RepeatCount, &g.ExpectedCount); err != nil {
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
		`SELECT id, instruction_group_id, sort_order, stitch_id, count, into_stitch, repeat_count, notes
		 FROM stitch_entries WHERE instruction_group_id = ? ORDER BY sort_order`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.StitchEntry
	for rows.Next() {
		var e domain.StitchEntry
		if err := rows.Scan(&e.ID, &e.InstructionGroupID, &e.SortOrder, &e.StitchID,
			&e.Count, &e.IntoStitch, &e.RepeatCount, &e.Notes); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
