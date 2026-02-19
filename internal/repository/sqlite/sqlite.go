package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/msomdec/stitch-map-2/internal/repository/sqlite/migrations"
	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB and implements domain.Database.
// It owns SQLite-specific configuration and migrations.
type DB struct {
	SqlDB *sql.DB
}

// New opens a SQLite database at the given path and configures it for use.
// It enables WAL mode and foreign keys.
func New(dbPath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := sqlDB.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Enable foreign key enforcement.
	if _, err := sqlDB.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Set a reasonable connection pool for SQLite.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.PingContext(context.Background()); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{SqlDB: sqlDB}, nil
}

// Migrate applies all pending SQLite migrations.
func (db *DB) Migrate(ctx context.Context) error {
	return migrations.Run(ctx, db.SqlDB)
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.SqlDB.Close()
}
