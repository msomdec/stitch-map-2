package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/migrations"
	_ "modernc.org/sqlite"
)

func TestRunMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Enable foreign keys for consistency with production.
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	ctx := context.Background()

	// First run should apply all migrations.
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("first migration run: %v", err)
	}

	// Verify the users table exists by inserting a row.
	_, err = db.ExecContext(ctx,
		"INSERT INTO users (email, display_name, password_hash) VALUES (?, ?, ?)",
		"test@example.com", "Test User", "hash123",
	)
	if err != nil {
		t.Fatalf("insert into users: %v", err)
	}

	// Verify schema_migrations tracks the applied migration.
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one migration recorded in schema_migrations")
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Run migrations twice; second run should be a no-op.
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("second run (idempotent): %v", err)
	}

	// Verify only one migration entry exists (not duplicated).
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 migration record, got %d", count)
	}
}
