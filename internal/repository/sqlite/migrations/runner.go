package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
)

// Run applies all unapplied migrations from the embedded FS to the database.
// It tracks applied migrations in a schema_migrations table.
func Run(ctx context.Context, db *sql.DB) error {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	applied, err := getAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("get applied migrations: %w", err)
	}

	files, err := listMigrationFiles()
	if err != nil {
		return fmt.Errorf("list migration files: %w", err)
	}

	for _, filename := range files {
		if applied[filename] {
			slog.Debug("migration already applied", "file", filename)
			continue
		}

		if err := applyMigration(ctx, db, filename); err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}
		slog.Info("migration applied", "file", filename)
	}

	return nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func getAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT filename FROM schema_migrations ORDER BY filename")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, err
		}
		applied[filename] = true
	}
	return applied, rows.Err()
}

func listMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func applyMigration(ctx context.Context, db *sql.DB, filename string) error {
	content, err := fs.ReadFile(FS, filename)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("execute sql: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES (?)", filename); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}
