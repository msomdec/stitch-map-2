package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

func TestStitchRepository_Create(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	stitch := &domain.Stitch{
		Abbreviation: "tst",
		Name:         "Test Stitch",
		Description:  "A test stitch",
		Category:     "basic",
	}

	if err := repo.Create(ctx, stitch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if stitch.ID == 0 {
		t.Fatal("expected stitch ID to be set")
	}
	if stitch.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestStitchRepository_Create_CustomStitch(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	userRepo := db.Users()
	ctx := context.Background()

	user := &domain.User{Email: "stitch@example.com", DisplayName: "Stitcher", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stitch := &domain.Stitch{
		Abbreviation: "msc",
		Name:         "My Stitch",
		Category:     "custom",
		IsCustom:     true,
		UserID:       &user.ID,
	}

	if err := repo.Create(ctx, stitch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if stitch.ID == 0 {
		t.Fatal("expected stitch ID to be set")
	}
	if !stitch.IsCustom {
		t.Fatal("expected IsCustom to be true")
	}
}

func TestStitchRepository_Create_DuplicateAbbreviation(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	userRepo := db.Users()
	ctx := context.Background()

	// Duplicate abbreviation for the same user triggers unique constraint.
	user := &domain.User{Email: "dup@example.com", DisplayName: "Dup", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	s1 := &domain.Stitch{Abbreviation: "dup", Name: "Stitch 1", Category: "custom", IsCustom: true, UserID: &user.ID}
	if err := repo.Create(ctx, s1); err != nil {
		t.Fatalf("Create s1: %v", err)
	}

	s2 := &domain.Stitch{Abbreviation: "dup", Name: "Stitch 2", Category: "custom", IsCustom: true, UserID: &user.ID}
	err := repo.Create(ctx, s2)
	if !errors.Is(err, domain.ErrDuplicateAbbreviation) {
		t.Fatalf("expected ErrDuplicateAbbreviation, got %v", err)
	}
}

func TestStitchRepository_SameAbbreviationDifferentUsers(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	userRepo := db.Users()
	ctx := context.Background()

	u1 := &domain.User{Email: "u1@example.com", DisplayName: "U1", PasswordHash: "hash"}
	u2 := &domain.User{Email: "u2@example.com", DisplayName: "U2", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, u1); err != nil {
		t.Fatalf("Create u1: %v", err)
	}
	if err := userRepo.Create(ctx, u2); err != nil {
		t.Fatalf("Create u2: %v", err)
	}

	s1 := &domain.Stitch{Abbreviation: "same", Name: "S1", Category: "custom", IsCustom: true, UserID: &u1.ID}
	s2 := &domain.Stitch{Abbreviation: "same", Name: "S2", Category: "custom", IsCustom: true, UserID: &u2.ID}

	if err := repo.Create(ctx, s1); err != nil {
		t.Fatalf("Create s1: %v", err)
	}
	if err := repo.Create(ctx, s2); err != nil {
		t.Fatalf("Create s2: %v", err)
	}
}

func TestStitchRepository_GetByID(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	stitch := &domain.Stitch{Abbreviation: "byid", Name: "By ID", Category: "basic"}
	if err := repo.Create(ctx, stitch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetByID(ctx, stitch.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.Abbreviation != "byid" {
		t.Fatalf("expected abbreviation 'byid', got %q", found.Abbreviation)
	}
	if found.Name != "By ID" {
		t.Fatalf("expected name 'By ID', got %q", found.Name)
	}
}

func TestStitchRepository_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 99999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStitchRepository_GetByAbbreviation(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	stitch := &domain.Stitch{Abbreviation: "abbr", Name: "Abbreviation Test", Category: "basic"}
	if err := repo.Create(ctx, stitch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetByAbbreviation(ctx, "abbr", nil)
	if err != nil {
		t.Fatalf("GetByAbbreviation: %v", err)
	}
	if found.ID != stitch.ID {
		t.Fatalf("expected ID %d, got %d", stitch.ID, found.ID)
	}
}

func TestStitchRepository_GetByAbbreviation_WithUserID(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	userRepo := db.Users()
	ctx := context.Background()

	user := &domain.User{Email: "abbr@example.com", DisplayName: "Abbr", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stitch := &domain.Stitch{Abbreviation: "cust", Name: "Custom", Category: "custom", IsCustom: true, UserID: &user.ID}
	if err := repo.Create(ctx, stitch); err != nil {
		t.Fatalf("Create stitch: %v", err)
	}

	found, err := repo.GetByAbbreviation(ctx, "cust", &user.ID)
	if err != nil {
		t.Fatalf("GetByAbbreviation with userID: %v", err)
	}
	if found.ID != stitch.ID {
		t.Fatalf("expected ID %d, got %d", stitch.ID, found.ID)
	}

	// Should not find with nil userID.
	_, err = repo.GetByAbbreviation(ctx, "cust", nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for nil userID, got %v", err)
	}
}

func TestStitchRepository_GetByAbbreviation_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	_, err := repo.GetByAbbreviation(ctx, "nonexistent", nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStitchRepository_ListPredefined(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	userRepo := db.Users()
	ctx := context.Background()

	// Create predefined and custom stitches.
	s1 := &domain.Stitch{Abbreviation: "pre1", Name: "Predefined 1", Category: "basic"}
	s2 := &domain.Stitch{Abbreviation: "pre2", Name: "Predefined 2", Category: "basic"}
	if err := repo.Create(ctx, s1); err != nil {
		t.Fatalf("Create s1: %v", err)
	}
	if err := repo.Create(ctx, s2); err != nil {
		t.Fatalf("Create s2: %v", err)
	}

	user := &domain.User{Email: "list@example.com", DisplayName: "List", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	s3 := &domain.Stitch{Abbreviation: "cust1", Name: "Custom 1", Category: "custom", IsCustom: true, UserID: &user.ID}
	if err := repo.Create(ctx, s3); err != nil {
		t.Fatalf("Create s3: %v", err)
	}

	predefined, err := repo.ListPredefined(ctx)
	if err != nil {
		t.Fatalf("ListPredefined: %v", err)
	}
	if len(predefined) != 2 {
		t.Fatalf("expected 2 predefined, got %d", len(predefined))
	}
	for _, s := range predefined {
		if s.IsCustom {
			t.Fatal("predefined list should not contain custom stitches")
		}
	}
}

func TestStitchRepository_ListByUser(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	userRepo := db.Users()
	ctx := context.Background()

	u1 := &domain.User{Email: "u1@example.com", DisplayName: "U1", PasswordHash: "hash"}
	u2 := &domain.User{Email: "u2@example.com", DisplayName: "U2", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, u1); err != nil {
		t.Fatalf("Create u1: %v", err)
	}
	if err := userRepo.Create(ctx, u2); err != nil {
		t.Fatalf("Create u2: %v", err)
	}

	s1 := &domain.Stitch{Abbreviation: "u1s", Name: "U1 Stitch", Category: "custom", IsCustom: true, UserID: &u1.ID}
	s2 := &domain.Stitch{Abbreviation: "u2s", Name: "U2 Stitch", Category: "custom", IsCustom: true, UserID: &u2.ID}
	if err := repo.Create(ctx, s1); err != nil {
		t.Fatalf("Create s1: %v", err)
	}
	if err := repo.Create(ctx, s2); err != nil {
		t.Fatalf("Create s2: %v", err)
	}

	user1Stitches, err := repo.ListByUser(ctx, u1.ID)
	if err != nil {
		t.Fatalf("ListByUser u1: %v", err)
	}
	if len(user1Stitches) != 1 {
		t.Fatalf("expected 1 stitch for u1, got %d", len(user1Stitches))
	}
	if user1Stitches[0].Abbreviation != "u1s" {
		t.Fatalf("expected abbreviation 'u1s', got %q", user1Stitches[0].Abbreviation)
	}
}

func TestStitchRepository_Update(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	stitch := &domain.Stitch{Abbreviation: "upd", Name: "Update Me", Category: "basic"}
	if err := repo.Create(ctx, stitch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	stitch.Name = "Updated"
	stitch.Description = "Now updated"
	if err := repo.Update(ctx, stitch); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := repo.GetByID(ctx, stitch.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.Name != "Updated" {
		t.Fatalf("expected name 'Updated', got %q", found.Name)
	}
	if found.Description != "Now updated" {
		t.Fatalf("expected description 'Now updated', got %q", found.Description)
	}
}

func TestStitchRepository_Update_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	stitch := &domain.Stitch{ID: 99999, Abbreviation: "nf", Name: "Not Found"}
	err := repo.Update(ctx, stitch)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStitchRepository_Delete(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	stitch := &domain.Stitch{Abbreviation: "del", Name: "Delete Me", Category: "basic"}
	if err := repo.Create(ctx, stitch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, stitch.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetByID(ctx, stitch.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestStitchRepository_Delete_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := db.Stitches()
	ctx := context.Background()

	err := repo.Delete(ctx, 99999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
