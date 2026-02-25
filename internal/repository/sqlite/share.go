package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

type shareRepo struct {
	db *sql.DB
}

func (r *shareRepo) Create(ctx context.Context, share *domain.PatternShare) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO pattern_shares (pattern_id, token, share_type, recipient_email)
		 VALUES (?, ?, ?, ?)`,
		share.PatternID, share.Token, share.ShareType, share.RecipientEmail,
	)
	if err != nil {
		return fmt.Errorf("insert pattern share: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get share id: %w", err)
	}
	share.ID = id
	return nil
}

func (r *shareRepo) GetByToken(ctx context.Context, token string) (*domain.PatternShare, error) {
	s := &domain.PatternShare{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, pattern_id, token, share_type, recipient_email, created_at
		 FROM pattern_shares WHERE token = ?`, token,
	).Scan(&s.ID, &s.PatternID, &s.Token, &s.ShareType, &s.RecipientEmail, &s.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get share by token: %w", err)
	}
	return s, nil
}

func (r *shareRepo) ListByPattern(ctx context.Context, patternID int64) ([]domain.PatternShare, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pattern_id, token, share_type, recipient_email, created_at
		 FROM pattern_shares WHERE pattern_id = ? ORDER BY created_at DESC`, patternID)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	defer rows.Close()

	var shares []domain.PatternShare
	for rows.Next() {
		var s domain.PatternShare
		if err := rows.Scan(&s.ID, &s.PatternID, &s.Token, &s.ShareType, &s.RecipientEmail, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan share: %w", err)
		}
		shares = append(shares, s)
	}
	return shares, rows.Err()
}

func (r *shareRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM pattern_shares WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete share: %w", err)
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

func (r *shareRepo) DeleteAllByPattern(ctx context.Context, patternID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM pattern_shares WHERE pattern_id = ?", patternID)
	if err != nil {
		return fmt.Errorf("delete all shares: %w", err)
	}
	return nil
}

func (r *shareRepo) HasSharesByPatternIDs(ctx context.Context, patternIDs []int64) (map[int64]bool, error) {
	if len(patternIDs) == 0 {
		return map[int64]bool{}, nil
	}

	placeholders := make([]string, len(patternIDs))
	args := make([]interface{}, len(patternIDs))
	for i, id := range patternIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT DISTINCT pattern_id FROM pattern_shares WHERE pattern_id IN (%s)`,
		strings.Join(placeholders, ","),
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("has shares by pattern ids: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]bool)
	for rows.Next() {
		var pid int64
		if err := rows.Scan(&pid); err != nil {
			return nil, fmt.Errorf("scan pattern id: %w", err)
		}
		result[pid] = true
	}
	return result, rows.Err()
}
