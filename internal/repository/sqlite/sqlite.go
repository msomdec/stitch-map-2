package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite/migrations"
	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB and implements domain.Database.
// It owns SQLite-specific configuration and migrations.
// All repository interfaces are accessed via factory methods on DB, so the
// entire database backend can be swapped by replacing a single sqlite.New call.
type DB struct {
	SqlDB *sql.DB
}

// Compile-time interface compliance checks.
var (
	_ domain.Database               = (*DB)(nil)
	_ domain.UserRepository         = (*userRepo)(nil)
	_ domain.StitchRepository       = (*stitchRepo)(nil)
	_ domain.PatternRepository      = (*patternRepo)(nil)
	_ domain.WorkSessionRepository  = (*workSessionRepo)(nil)
	_ domain.PatternImageRepository = (*patternImageRepo)(nil)
	_ domain.FileStore              = (*fileStore)(nil)
	_ domain.PatternShareRepository = (*shareRepo)(nil)
)

// Users returns a domain.UserRepository backed by this database.
func (db *DB) Users() domain.UserRepository { return &userRepo{db: db.SqlDB} }

// Stitches returns a domain.StitchRepository backed by this database.
func (db *DB) Stitches() domain.StitchRepository { return &stitchRepo{db: db.SqlDB} }

// Patterns returns a domain.PatternRepository backed by this database.
func (db *DB) Patterns() domain.PatternRepository { return &patternRepo{db: db.SqlDB} }

// Sessions returns a domain.WorkSessionRepository backed by this database.
func (db *DB) Sessions() domain.WorkSessionRepository { return &workSessionRepo{db: db.SqlDB} }

// PatternImages returns a domain.PatternImageRepository backed by this database.
func (db *DB) PatternImages() domain.PatternImageRepository { return &patternImageRepo{db: db.SqlDB} }

// FileStore returns a domain.FileStore backed by SQLite BLOBs.
func (db *DB) FileStore() domain.FileStore { return &fileStore{db: db.SqlDB} }

// Shares returns a domain.PatternShareRepository backed by this database.
func (db *DB) Shares() domain.PatternShareRepository { return &shareRepo{db: db.SqlDB} }

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
