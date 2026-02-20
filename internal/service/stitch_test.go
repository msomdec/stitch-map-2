package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
	"github.com/msomdec/stitch-map-2/internal/service"
)

func newTestStitchService(t *testing.T) (*service.StitchService, *sqlite.DB) {
	t.Helper()
	_, db := newTestAuthService(t)
	stitchRepo := sqlite.NewStitchRepository(db)
	return service.NewStitchService(stitchRepo), db
}

func TestStitchService_SeedPredefined(t *testing.T) {
	svc, _ := newTestStitchService(t)
	ctx := context.Background()

	if err := svc.SeedPredefined(ctx); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	predefined, err := svc.ListPredefined(ctx)
	if err != nil {
		t.Fatalf("ListPredefined: %v", err)
	}

	// There should be 34 predefined stitches as defined in service/stitch.go.
	if len(predefined) != 34 {
		t.Fatalf("expected 34 predefined stitches, got %d", len(predefined))
	}
}

func TestStitchService_SeedPredefined_Idempotent(t *testing.T) {
	svc, _ := newTestStitchService(t)
	ctx := context.Background()

	// Seed twice.
	if err := svc.SeedPredefined(ctx); err != nil {
		t.Fatalf("first SeedPredefined: %v", err)
	}
	if err := svc.SeedPredefined(ctx); err != nil {
		t.Fatalf("second SeedPredefined: %v", err)
	}

	predefined, err := svc.ListPredefined(ctx)
	if err != nil {
		t.Fatalf("ListPredefined: %v", err)
	}

	if len(predefined) != 34 {
		t.Fatalf("expected 34 predefined stitches after double seed, got %d", len(predefined))
	}
}

func TestStitchService_CreateCustom_Success(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	// Create a user first.
	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "custom@example.com", DisplayName: "Custom", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stitch, err := svc.CreateCustom(ctx, user.ID, "msc", "My Special Crochet", "A custom stitch", "custom")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}

	if stitch.ID == 0 {
		t.Fatal("expected stitch ID to be set")
	}
	if !stitch.IsCustom {
		t.Fatal("expected IsCustom to be true")
	}
	if stitch.UserID == nil || *stitch.UserID != user.ID {
		t.Fatal("expected UserID to match")
	}
	if stitch.Category != "custom" {
		t.Fatalf("expected category 'custom', got %q", stitch.Category)
	}
}

func TestStitchService_CreateCustom_DefaultCategory(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "defcat@example.com", DisplayName: "DefCat", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stitch, err := svc.CreateCustom(ctx, user.ID, "dfc", "Default Category", "", "")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}
	if stitch.Category != "custom" {
		t.Fatalf("expected default category 'custom', got %q", stitch.Category)
	}
}

func TestStitchService_CreateCustom_RejectReservedAbbreviation(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	// Seed predefined stitches.
	if err := svc.SeedPredefined(ctx); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "reserved@example.com", DisplayName: "Reserved", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	// Try to create a custom stitch with a predefined abbreviation "sc".
	_, err := svc.CreateCustom(ctx, user.ID, "sc", "My SC", "", "")
	if !errors.Is(err, domain.ErrReservedAbbreviation) {
		t.Fatalf("expected ErrReservedAbbreviation, got %v", err)
	}
}

func TestStitchService_CreateCustom_InvalidInput(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "invalid@example.com", DisplayName: "Invalid", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	tests := []struct {
		name         string
		abbreviation string
		stitchName   string
	}{
		{"empty abbreviation", "", "Some Name"},
		{"empty name", "sn", ""},
		{"both empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateCustom(ctx, user.ID, tc.abbreviation, tc.stitchName, "", "")
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestStitchService_ListAll(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	if err := svc.SeedPredefined(ctx); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "listall@example.com", DisplayName: "ListAll", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	// Create a custom stitch.
	_, err := svc.CreateCustom(ctx, user.ID, "xyz", "XYZ Stitch", "", "")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}

	all, err := svc.ListAll(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	// 34 predefined + 1 custom.
	if len(all) != 35 {
		t.Fatalf("expected 35 stitches, got %d", len(all))
	}
}

func TestStitchService_UpdateCustom_Success(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "update@example.com", DisplayName: "Update", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stitch, err := svc.CreateCustom(ctx, user.ID, "upd", "Update Me", "", "custom")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}

	updated, err := svc.UpdateCustom(ctx, user.ID, stitch.ID, "upd2", "Updated Name", "New desc", "specialty")
	if err != nil {
		t.Fatalf("UpdateCustom: %v", err)
	}
	if updated.Abbreviation != "upd2" {
		t.Fatalf("expected abbreviation 'upd2', got %q", updated.Abbreviation)
	}
	if updated.Name != "Updated Name" {
		t.Fatalf("expected name 'Updated Name', got %q", updated.Name)
	}
	if updated.Category != "specialty" {
		t.Fatalf("expected category 'specialty', got %q", updated.Category)
	}
}

func TestStitchService_UpdateCustom_WrongUser(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	userRepo := sqlite.NewUserRepository(db)
	u1 := &domain.User{Email: "owner@example.com", DisplayName: "Owner", PasswordHash: "hash"}
	u2 := &domain.User{Email: "other@example.com", DisplayName: "Other", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, u1); err != nil {
		t.Fatalf("Create u1: %v", err)
	}
	if err := userRepo.Create(ctx, u2); err != nil {
		t.Fatalf("Create u2: %v", err)
	}

	stitch, err := svc.CreateCustom(ctx, u1.ID, "own", "Owner Stitch", "", "")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}

	// Other user tries to update.
	_, err = svc.UpdateCustom(ctx, u2.ID, stitch.ID, "own", "Hacked", "", "")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestStitchService_UpdateCustom_RejectReservedAbbreviation(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	if err := svc.SeedPredefined(ctx); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "updres@example.com", DisplayName: "UpdRes", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stitch, err := svc.CreateCustom(ctx, user.ID, "orig", "Original", "", "")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}

	// Try to change abbreviation to a predefined one.
	_, err = svc.UpdateCustom(ctx, user.ID, stitch.ID, "sc", "Renamed", "", "")
	if !errors.Is(err, domain.ErrReservedAbbreviation) {
		t.Fatalf("expected ErrReservedAbbreviation, got %v", err)
	}
}

func TestStitchService_DeleteCustom_Success(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	userRepo := sqlite.NewUserRepository(db)
	user := &domain.User{Email: "delete@example.com", DisplayName: "Delete", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	stitch, err := svc.CreateCustom(ctx, user.ID, "del", "Delete Me", "", "")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}

	if err := svc.DeleteCustom(ctx, user.ID, stitch.ID); err != nil {
		t.Fatalf("DeleteCustom: %v", err)
	}

	// Verify it's gone.
	_, err = svc.GetByID(ctx, stitch.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestStitchService_DeleteCustom_WrongUser(t *testing.T) {
	svc, db := newTestStitchService(t)
	ctx := context.Background()

	userRepo := sqlite.NewUserRepository(db)
	u1 := &domain.User{Email: "delown@example.com", DisplayName: "Owner", PasswordHash: "hash"}
	u2 := &domain.User{Email: "delother@example.com", DisplayName: "Other", PasswordHash: "hash"}
	if err := userRepo.Create(ctx, u1); err != nil {
		t.Fatalf("Create u1: %v", err)
	}
	if err := userRepo.Create(ctx, u2); err != nil {
		t.Fatalf("Create u2: %v", err)
	}

	stitch, err := svc.CreateCustom(ctx, u1.ID, "own2", "Owner Stitch", "", "")
	if err != nil {
		t.Fatalf("CreateCustom: %v", err)
	}

	err = svc.DeleteCustom(ctx, u2.ID, stitch.ID)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}
