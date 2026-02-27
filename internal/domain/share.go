package domain

import (
	"context"
	"time"
)

type ShareType string

const (
	ShareTypeGlobal ShareType = "global"
	ShareTypeEmail  ShareType = "email"
)

type PatternShare struct {
	ID             int64
	PatternID      int64
	Token          string
	ShareType      ShareType
	RecipientEmail string
	CreatedAt      time.Time
}

type PatternShareRepository interface {
	Create(ctx context.Context, share *PatternShare) error
	GetByID(ctx context.Context, id int64) (*PatternShare, error)
	GetByToken(ctx context.Context, token string) (*PatternShare, error)
	ListByPattern(ctx context.Context, patternID int64) ([]PatternShare, error)
	Delete(ctx context.Context, id int64) error
	DeleteAllByPattern(ctx context.Context, patternID int64) error
	HasSharesByPatternIDs(ctx context.Context, patternIDs []int64) (map[int64]bool, error)
}
