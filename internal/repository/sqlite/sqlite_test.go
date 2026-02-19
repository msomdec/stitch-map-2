package sqlite_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
)

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
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Verify foreign keys are enabled.
	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("check foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Fatalf("expected foreign_keys=1, got %d", fkEnabled)
	}
}
