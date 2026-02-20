package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
)

func newTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUserRepository_Create(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewUserRepository(db)
	ctx := context.Background()

	user := &domain.User{
		Email:        "test@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hashedpw",
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if user.ID == 0 {
		t.Fatal("expected user ID to be set after create")
	}
	if user.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewUserRepository(db)
	ctx := context.Background()

	user1 := &domain.User{
		Email:        "dup@example.com",
		DisplayName:  "User 1",
		PasswordHash: "hash1",
	}
	if err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("Create user1: %v", err)
	}

	user2 := &domain.User{
		Email:        "dup@example.com",
		DisplayName:  "User 2",
		PasswordHash: "hash2",
	}
	err := repo.Create(ctx, user2)
	if !errors.Is(err, domain.ErrDuplicateEmail) {
		t.Fatalf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewUserRepository(db)
	ctx := context.Background()

	user := &domain.User{
		Email:        "byid@example.com",
		DisplayName:  "By ID",
		PasswordHash: "hash",
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if found.Email != user.Email {
		t.Fatalf("expected email %q, got %q", user.Email, found.Email)
	}
	if found.DisplayName != user.DisplayName {
		t.Fatalf("expected display name %q, got %q", user.DisplayName, found.DisplayName)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 99999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewUserRepository(db)
	ctx := context.Background()

	user := &domain.User{
		Email:        "byemail@example.com",
		DisplayName:  "By Email",
		PasswordHash: "hash",
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetByEmail(ctx, "byemail@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}

	if found.ID != user.ID {
		t.Fatalf("expected id %d, got %d", user.ID, found.ID)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlite.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
