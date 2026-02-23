package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// fileStore implements domain.FileStore using SQLite BLOBs.
type fileStore struct {
	db *sql.DB
}

func (s *fileStore) Save(ctx context.Context, key string, data []byte) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO file_blobs (storage_key, data) VALUES (?, ?)",
		key, data,
	)
	if err != nil {
		return fmt.Errorf("save file blob: %w", err)
	}
	return nil
}

func (s *fileStore) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT data FROM file_blobs WHERE storage_key = ?", key,
	).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get file blob: %w", err)
	}
	return data, nil
}

func (s *fileStore) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM file_blobs WHERE storage_key = ?", key,
	)
	if err != nil {
		return fmt.Errorf("delete file blob: %w", err)
	}
	return nil
}
