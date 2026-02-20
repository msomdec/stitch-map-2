package domain

import (
	"context"
	"time"
)

// Stitch represents a crochet stitch type, either predefined or user-created.
type Stitch struct {
	ID           int64
	Abbreviation string
	Name         string
	Description  string
	Category     string // "basic", "advanced", "decrease", "increase", "post", "specialty", "action"
	IsCustom     bool
	UserID       *int64
	CreatedAt    time.Time
}

// StitchRepository defines persistence operations for stitches.
type StitchRepository interface {
	ListPredefined(ctx context.Context) ([]Stitch, error)
	ListByUser(ctx context.Context, userID int64) ([]Stitch, error)
	GetByID(ctx context.Context, id int64) (*Stitch, error)
	GetByAbbreviation(ctx context.Context, abbreviation string, userID *int64) (*Stitch, error)
	Create(ctx context.Context, stitch *Stitch) error
	Update(ctx context.Context, stitch *Stitch) error
	Delete(ctx context.Context, id int64) error
}
