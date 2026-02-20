package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// WorkSessionRepository implements domain.WorkSessionRepository using SQLite.
type WorkSessionRepository struct {
	db *sql.DB
}

// NewWorkSessionRepository creates a new SQLite-backed WorkSessionRepository.
func NewWorkSessionRepository(db *DB) *WorkSessionRepository {
	return &WorkSessionRepository{db: db.SqlDB}
}

func (r *WorkSessionRepository) Create(ctx context.Context, session *domain.WorkSession) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO work_sessions (pattern_id, user_id, current_group_index, current_group_repeat,
		 current_stitch_index, current_stitch_repeat, current_stitch_count, status, started_at, last_activity_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.PatternID, session.UserID,
		session.CurrentGroupIndex, session.CurrentGroupRepeat,
		session.CurrentStitchIndex, session.CurrentStitchRepeat, session.CurrentStitchCount,
		session.Status, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert work session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get work session id: %w", err)
	}

	session.ID = id
	session.StartedAt = now
	session.LastActivityAt = now
	return nil
}

func (r *WorkSessionRepository) GetByID(ctx context.Context, id int64) (*domain.WorkSession, error) {
	s := &domain.WorkSession{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, pattern_id, user_id, current_group_index, current_group_repeat,
		 current_stitch_index, current_stitch_repeat, current_stitch_count,
		 status, started_at, last_activity_at, completed_at
		 FROM work_sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.PatternID, &s.UserID,
		&s.CurrentGroupIndex, &s.CurrentGroupRepeat,
		&s.CurrentStitchIndex, &s.CurrentStitchRepeat, &s.CurrentStitchCount,
		&s.Status, &s.StartedAt, &s.LastActivityAt, &s.CompletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get work session: %w", err)
	}
	return s, nil
}

func (r *WorkSessionRepository) GetActiveByUser(ctx context.Context, userID int64) ([]domain.WorkSession, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ws.id, ws.pattern_id, ws.user_id, ws.current_group_index, ws.current_group_repeat,
		 ws.current_stitch_index, ws.current_stitch_repeat, ws.current_stitch_count,
		 ws.status, ws.started_at, ws.last_activity_at, ws.completed_at
		 FROM work_sessions ws
		 WHERE ws.user_id = ? AND ws.status IN ('active', 'paused')
		 ORDER BY ws.last_activity_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.WorkSession
	for rows.Next() {
		var s domain.WorkSession
		if err := rows.Scan(&s.ID, &s.PatternID, &s.UserID,
			&s.CurrentGroupIndex, &s.CurrentGroupRepeat,
			&s.CurrentStitchIndex, &s.CurrentStitchRepeat, &s.CurrentStitchCount,
			&s.Status, &s.StartedAt, &s.LastActivityAt, &s.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan work session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *WorkSessionRepository) Update(ctx context.Context, session *domain.WorkSession) error {
	now := time.Now().UTC()
	session.LastActivityAt = now

	result, err := r.db.ExecContext(ctx,
		`UPDATE work_sessions SET
		 current_group_index = ?, current_group_repeat = ?,
		 current_stitch_index = ?, current_stitch_repeat = ?, current_stitch_count = ?,
		 status = ?, last_activity_at = ?, completed_at = ?
		 WHERE id = ?`,
		session.CurrentGroupIndex, session.CurrentGroupRepeat,
		session.CurrentStitchIndex, session.CurrentStitchRepeat, session.CurrentStitchCount,
		session.Status, now, session.CompletedAt, session.ID,
	)
	if err != nil {
		return fmt.Errorf("update work session: %w", err)
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

func (r *WorkSessionRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM work_sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete work session: %w", err)
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
