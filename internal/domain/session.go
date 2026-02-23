package domain

import (
	"context"
	"time"
)

// WorkSession tracks a user's real-time progress through a specific pattern.
type WorkSession struct {
	ID                  int64
	PatternID           int64
	UserID              int64
	CurrentGroupIndex   int // Which instruction group the user is on (0-based)
	CurrentGroupRepeat  int // Which repeat of the group they're on (0-based)
	CurrentStitchIndex  int // Which stitch entry within the group (0-based)
	CurrentStitchRepeat int // Which repeat of the stitch entry (0-based)
	CurrentStitchCount  int // Which individual stitch within the count (0-based)
	Status              string
	StartedAt           time.Time
	LastActivityAt      time.Time
	CompletedAt         *time.Time
}

const (
	SessionStatusActive    = "active"
	SessionStatusPaused    = "paused"
	SessionStatusCompleted = "completed"
)

type WorkSessionRepository interface {
	Create(ctx context.Context, session *WorkSession) error
	GetByID(ctx context.Context, id int64) (*WorkSession, error)
	GetActiveByUser(ctx context.Context, userID int64) ([]WorkSession, error)
	GetCompletedByUser(ctx context.Context, userID int64, limit, offset int) ([]WorkSession, error)
	CountCompletedByUser(ctx context.Context, userID int64) (int, error)
	Update(ctx context.Context, session *WorkSession) error
	Delete(ctx context.Context, id int64) error
}
