package domain

import (
	"context"
	"time"
)

type PatternType string

const (
	PatternTypeRound PatternType = "round"
	PatternTypeRow   PatternType = "row"
)

type Pattern struct {
	ID                int64
	UserID            int64
	Name              string
	Description       string
	PatternType       PatternType
	HookSize          string
	YarnWeight        string
	Difficulty        string
	Locked            bool
	SharedFromUserID  *int64
	SharedFromName    string
	PatternStitches   []PatternStitch
	InstructionGroups []InstructionGroup
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PatternStitch struct {
	ID              int64
	PatternID       int64
	Abbreviation    string
	Name            string
	Description     string
	Category        string
	LibraryStitchID *int64
}

type InstructionGroup struct {
	ID            int64
	PatternID     int64
	SortOrder     int
	Label         string
	RepeatCount   int
	StitchEntries []StitchEntry
	ExpectedCount *int
	Notes         string
}

type StitchEntry struct {
	ID                 int64
	InstructionGroupID int64
	SortOrder          int
	PatternStitchID    int64
	Count              int
	IntoStitch         string
	RepeatCount        int
}

// PatternFilter holds search/filter/sort options for pattern listing.
type PatternFilter struct {
	Query      string // Text search on name and description
	Type       string // "round", "row", or "" for all
	Difficulty string // "Beginner", "Intermediate", "Advanced", "Expert", or "" for all
	Sort       string // "updated" (default), "name", "created", "stitches"
}

// PatternSummary holds lightweight pattern metadata for list views.
type PatternSummary struct {
	ID               int64
	UserID           int64
	Name             string
	Description      string
	PatternType      PatternType
	HookSize         string
	YarnWeight       string
	Difficulty       string
	Locked           bool
	SharedFromUserID *int64
	SharedFromName   string
	GroupCount       int
	StitchCount      int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PatternRepository interface {
	Create(ctx context.Context, pattern *Pattern) error
	GetByID(ctx context.Context, id int64) (*Pattern, error)
	GetNamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
	ListByUser(ctx context.Context, userID int64) ([]Pattern, error)
	ListSharedWithUser(ctx context.Context, userID int64) ([]Pattern, error)
	ListSummaryByUser(ctx context.Context, userID int64) ([]PatternSummary, error)
	ListSummarySharedWithUser(ctx context.Context, userID int64) ([]PatternSummary, error)
	SearchSummaryByUser(ctx context.Context, userID int64, filter PatternFilter) ([]PatternSummary, error)
	Update(ctx context.Context, pattern *Pattern) error
	Delete(ctx context.Context, id int64) error
	Duplicate(ctx context.Context, id int64, newUserID int64) (*Pattern, error)
	DuplicateAsShared(ctx context.Context, id int64, newUserID int64, sharedFromUserID int64, sharedFromName string) (*Pattern, error)
}
