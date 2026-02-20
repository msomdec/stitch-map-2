package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// userRepo implements domain.UserRepository using SQLite.
type userRepo struct {
	db *sql.DB
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		user.Email, user.DisplayName, user.PasswordHash, now, now,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return domain.ErrDuplicateEmail
		}
		return fmt.Errorf("insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	user.ID = id
	user.CreatedAt = now
	user.UpdatedAt = now
	return nil
}

func (r *userRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, display_name, password_hash, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}
	return user, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, display_name, password_hash, created_at, updated_at
		 FROM users WHERE email = ?`, email,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("query user by email: %w", err)
	}
	return user, nil
}

// isUniqueConstraintError checks if the error is a SQLite unique constraint violation.
func isUniqueConstraintError(err error) bool {
	return err != nil && (errors.Is(err, sql.ErrNoRows) == false) &&
		(containsString(err.Error(), "UNIQUE constraint failed") ||
			containsString(err.Error(), "unique constraint"))
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
