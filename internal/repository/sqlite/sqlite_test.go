package sqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
)

// Verify that *sqlite.DB implements domain.Database at compile time.
var _ domain.Database = (*sqlite.DB)(nil)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	// Verify the file was created.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Verify we can ping the database.
	if err := db.SqlDB.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Verify foreign keys are enabled.
	var fkEnabled int
	if err := db.SqlDB.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("check foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Fatalf("expected foreign_keys=1, got %d", fkEnabled)
	}
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Run migrations through the Database interface.
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Verify the users table exists by inserting a row.
	_, err = db.SqlDB.ExecContext(ctx,
		"INSERT INTO users (email, display_name, password_hash) VALUES (?, ?, ?)",
		"test@example.com", "Test User", "hash123",
	)
	if err != nil {
		t.Fatalf("insert into users: %v", err)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Run migrations twice; second run should be a no-op.
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate (idempotent): %v", err)
	}

	// Verify only one migration entry exists.
	var count int
	err = db.SqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected 7 migration records, got %d", count)
	}
}
