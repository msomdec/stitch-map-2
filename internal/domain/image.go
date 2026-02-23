package domain

import (
	"context"
	"time"
)

// PatternImage holds metadata about an image attached to an instruction group.
type PatternImage struct {
	ID                 int64
	InstructionGroupID int64
	Filename           string // Original upload filename
	ContentType        string // "image/jpeg" or "image/png"
	Size               int64  // File size in bytes
	StorageKey         string // Key used to retrieve bytes from FileStore
	SortOrder          int    // Display order within the group
	CreatedAt          time.Time
}

// PatternImageRepository handles image metadata persistence.
type PatternImageRepository interface {
	Create(ctx context.Context, image *PatternImage) error
	GetByID(ctx context.Context, id int64) (*PatternImage, error)
	ListByGroup(ctx context.Context, groupID int64) ([]PatternImage, error)
	Delete(ctx context.Context, id int64) error
	CountByGroup(ctx context.Context, groupID int64) (int, error)
	// GetOwnerUserID returns the user ID of the pattern that owns the image,
	// resolved via instruction_groups → patterns. Used for ownership checks.
	GetOwnerUserID(ctx context.Context, imageID int64) (int64, error)
}

// FileStore abstracts raw file byte storage.
// The initial implementation stores BLOBs in SQLite; this interface
// allows swapping to filesystem, S3, or another backend later.
type FileStore interface {
	Save(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}
