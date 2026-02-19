package domain

import "context"

// Database defines lifecycle operations for the underlying database.
// Each implementation (SQLite, Postgres, etc.) owns its own migration
// files and strategy, ensuring the entire backend is swappable.
type Database interface {
	Migrate(ctx context.Context) error
	Close() error
}
