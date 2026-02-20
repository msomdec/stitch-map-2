package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// StitchRepository implements domain.StitchRepository using SQLite.
type StitchRepository struct {
	db *sql.DB
}

// NewStitchRepository creates a new SQLite-backed StitchRepository.
func NewStitchRepository(db *DB) *StitchRepository {
	return &StitchRepository{db: db.SqlDB}
}

func (r *StitchRepository) ListPredefined(ctx context.Context) ([]domain.Stitch, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, abbreviation, name, description, category, is_custom, user_id, created_at
		 FROM stitches WHERE is_custom = FALSE ORDER BY category, abbreviation`)
	if err != nil {
		return nil, fmt.Errorf("list predefined stitches: %w", err)
	}
	defer rows.Close()
	return scanStitches(rows)
}

func (r *StitchRepository) ListByUser(ctx context.Context, userID int64) ([]domain.Stitch, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, abbreviation, name, description, category, is_custom, user_id, created_at
		 FROM stitches WHERE is_custom = TRUE AND user_id = ? ORDER BY abbreviation`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user stitches: %w", err)
	}
	defer rows.Close()
	return scanStitches(rows)
}

func (r *StitchRepository) GetByID(ctx context.Context, id int64) (*domain.Stitch, error) {
	s := &domain.Stitch{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, abbreviation, name, description, category, is_custom, user_id, created_at
		 FROM stitches WHERE id = ?`, id,
	).Scan(&s.ID, &s.Abbreviation, &s.Name, &s.Description, &s.Category, &s.IsCustom, &s.UserID, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get stitch by id: %w", err)
	}
	return s, nil
}

func (r *StitchRepository) GetByAbbreviation(ctx context.Context, abbreviation string, userID *int64) (*domain.Stitch, error) {
	var row *sql.Row
	if userID == nil {
		row = r.db.QueryRowContext(ctx,
			`SELECT id, abbreviation, name, description, category, is_custom, user_id, created_at
			 FROM stitches WHERE abbreviation = ? AND user_id IS NULL`, abbreviation)
	} else {
		row = r.db.QueryRowContext(ctx,
			`SELECT id, abbreviation, name, description, category, is_custom, user_id, created_at
			 FROM stitches WHERE abbreviation = ? AND user_id = ?`, abbreviation, *userID)
	}

	s := &domain.Stitch{}
	err := row.Scan(&s.ID, &s.Abbreviation, &s.Name, &s.Description, &s.Category, &s.IsCustom, &s.UserID, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get stitch by abbreviation: %w", err)
	}
	return s, nil
}

func (r *StitchRepository) Create(ctx context.Context, stitch *domain.Stitch) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO stitches (abbreviation, name, description, category, is_custom, user_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		stitch.Abbreviation, stitch.Name, stitch.Description, stitch.Category,
		stitch.IsCustom, stitch.UserID, now,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return domain.ErrDuplicateAbbreviation
		}
		return fmt.Errorf("insert stitch: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	stitch.ID = id
	stitch.CreatedAt = now
	return nil
}

func (r *StitchRepository) Update(ctx context.Context, stitch *domain.Stitch) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE stitches SET abbreviation = ?, name = ?, description = ?, category = ?
		 WHERE id = ?`,
		stitch.Abbreviation, stitch.Name, stitch.Description, stitch.Category, stitch.ID,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return domain.ErrDuplicateAbbreviation
		}
		return fmt.Errorf("update stitch: %w", err)
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

func (r *StitchRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM stitches WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete stitch: %w", err)
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

func scanStitches(rows *sql.Rows) ([]domain.Stitch, error) {
	var stitches []domain.Stitch
	for rows.Next() {
		var s domain.Stitch
		if err := rows.Scan(&s.ID, &s.Abbreviation, &s.Name, &s.Description, &s.Category, &s.IsCustom, &s.UserID, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan stitch: %w", err)
		}
		stitches = append(stitches, s)
	}
	return stitches, rows.Err()
}
