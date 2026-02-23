package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// patternImageRepo implements domain.PatternImageRepository using SQLite.
type patternImageRepo struct {
	db *sql.DB
}

func (r *patternImageRepo) Create(ctx context.Context, image *domain.PatternImage) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO pattern_images (instruction_group_id, filename, content_type, size, storage_key, sort_order, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		image.InstructionGroupID, image.Filename, image.ContentType,
		image.Size, image.StorageKey, image.SortOrder, now,
	)
	if err != nil {
		return fmt.Errorf("insert pattern image: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	image.ID = id
	image.CreatedAt = now
	return nil
}

func (r *patternImageRepo) GetByID(ctx context.Context, id int64) (*domain.PatternImage, error) {
	img := &domain.PatternImage{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, instruction_group_id, filename, content_type, size, storage_key, sort_order, created_at
		 FROM pattern_images WHERE id = ?`, id,
	).Scan(&img.ID, &img.InstructionGroupID, &img.Filename, &img.ContentType,
		&img.Size, &img.StorageKey, &img.SortOrder, &img.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get pattern image: %w", err)
	}
	return img, nil
}

func (r *patternImageRepo) ListByGroup(ctx context.Context, groupID int64) ([]domain.PatternImage, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, instruction_group_id, filename, content_type, size, storage_key, sort_order, created_at
		 FROM pattern_images WHERE instruction_group_id = ? ORDER BY sort_order`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list pattern images: %w", err)
	}
	defer rows.Close()

	var images []domain.PatternImage
	for rows.Next() {
		var img domain.PatternImage
		if err := rows.Scan(&img.ID, &img.InstructionGroupID, &img.Filename, &img.ContentType,
			&img.Size, &img.StorageKey, &img.SortOrder, &img.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pattern image: %w", err)
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (r *patternImageRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM pattern_images WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete pattern image: %w", err)
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

func (r *patternImageRepo) CountByGroup(ctx context.Context, groupID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pattern_images WHERE instruction_group_id = ?", groupID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pattern images: %w", err)
	}
	return count, nil
}

func (r *patternImageRepo) GetOwnerUserID(ctx context.Context, imageID int64) (int64, error) {
	var userID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT p.user_id FROM pattern_images pi
		 JOIN instruction_groups ig ON pi.instruction_group_id = ig.id
		 JOIN patterns p ON ig.pattern_id = p.id
		 WHERE pi.id = ?`, imageID,
	).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, domain.ErrNotFound
		}
		return 0, fmt.Errorf("get image owner: %w", err)
	}
	return userID, nil
}
