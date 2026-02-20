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
	Notes             string
	InstructionGroups []InstructionGroup
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type InstructionGroup struct {
	ID            int64
	PatternID     int64
	SortOrder     int
	Label         string
	RepeatCount   int
	StitchEntries []StitchEntry
	ExpectedCount *int
}

type StitchEntry struct {
	ID                 int64
	InstructionGroupID int64
	SortOrder          int
	StitchID           int64
	Count              int
	IntoStitch         string
	RepeatCount        int
	Notes              string
}

type PatternRepository interface {
	Create(ctx context.Context, pattern *Pattern) error
	GetByID(ctx context.Context, id int64) (*Pattern, error)
	ListByUser(ctx context.Context, userID int64) ([]Pattern, error)
	Update(ctx context.Context, pattern *Pattern) error
	Delete(ctx context.Context, id int64) error
	Duplicate(ctx context.Context, id int64, newUserID int64) (*Pattern, error)
}
