package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
	"github.com/msomdec/stitch-map-2/internal/service"
)

func newTestPatternService(t *testing.T) (*service.PatternService, *service.StitchService, *sqlite.DB) {
	t.Helper()
	_, db := newTestAuthService(t)
	stitchRepo := db.Stitches()
	patternRepo := db.Patterns()
	return service.NewPatternService(patternRepo, stitchRepo), service.NewStitchService(stitchRepo), db
}

func seedStitchForTest(t *testing.T, db *sqlite.DB) int64 {
	t.Helper()
	repo := db.Stitches()
	s := &domain.Stitch{Abbreviation: "sc", Name: "Single Crochet", Category: "basic"}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("seed stitch: %v", err)
	}
	return s.ID
}

func seedUserForTest(t *testing.T, db *sqlite.DB, email string) int64 {
	t.Helper()
	repo := db.Users()
	u := &domain.User{Email: email, DisplayName: "Test", PasswordHash: "hash"}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u.ID
}

func TestPatternService_Create_Success(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "create@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      userID,
		Name:        "Test Pattern",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}

	if err := svc.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected pattern ID to be set")
	}

	// Verify pattern stitches were created.
	got, err := db.Patterns().GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.PatternStitches) != 1 {
		t.Fatalf("expected 1 pattern stitch, got %d", len(got.PatternStitches))
	}
	if got.PatternStitches[0].Abbreviation != "sc" {
		t.Fatalf("expected abbreviation 'sc', got %q", got.PatternStitches[0].Abbreviation)
	}
}

func TestPatternService_Create_EmptyName(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "emptyname@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      userID,
		Name:        "",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}

	err := svc.Create(ctx, p)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty name, got %v", err)
	}
}

func TestPatternService_Create_InvalidType(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "badtype@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      userID,
		Name:        "Bad Type",
		PatternType: "invalid",
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}

	err := svc.Create(ctx, p)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid type, got %v", err)
	}
}

func TestPatternService_Create_NoGroups(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "nogroups@example.com")

	p := &domain.Pattern{
		UserID:            userID,
		Name:              "No Groups",
		PatternType:       domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{},
	}

	err := svc.Create(ctx, p)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for no groups, got %v", err)
	}
}

func TestPatternService_Create_InvalidStitchID(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "badstitch@example.com")

	p := &domain.Pattern{
		UserID:      userID,
		Name:        "Bad Stitch Ref",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: 99999, Count: 6, RepeatCount: 1},
				}},
		},
	}

	err := svc.Create(ctx, p)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid stitch ID, got %v", err)
	}
}

func TestPatternService_Update_OwnershipCheck(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	u1 := seedUserForTest(t, db, "owner@example.com")
	u2 := seedUserForTest(t, db, "other@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      u1,
		Name:        "Owner Pattern",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}
	if err := svc.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Other user tries to update.
	p.Name = "Hacked"
	err := svc.Update(ctx, u2, p)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestPatternService_Delete_OwnershipCheck(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	u1 := seedUserForTest(t, db, "delowner@example.com")
	u2 := seedUserForTest(t, db, "delother@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      u1,
		Name:        "Del Owner",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}
	if err := svc.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := svc.Delete(ctx, u2, p.ID)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestPatternService_Update_Locked(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "locked-update@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      userID,
		Name:        "Lockable Pattern",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}
	if err := svc.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Lock the pattern directly in the DB.
	_, err := db.SqlDB.ExecContext(ctx, "UPDATE patterns SET locked = TRUE WHERE id = ?", p.ID)
	if err != nil {
		t.Fatalf("lock pattern: %v", err)
	}

	// Try to update the locked pattern.
	p.Name = "Updated Name"
	err = svc.Update(ctx, userID, p)
	if !errors.Is(err, domain.ErrPatternLocked) {
		t.Fatalf("expected ErrPatternLocked, got %v", err)
	}
}

func TestPatternService_Delete_Locked(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "locked-delete@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      userID,
		Name:        "Lockable Pattern",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}
	if err := svc.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Lock the pattern directly in the DB.
	_, err := db.SqlDB.ExecContext(ctx, "UPDATE patterns SET locked = TRUE WHERE id = ?", p.ID)
	if err != nil {
		t.Fatalf("lock pattern: %v", err)
	}

	// Try to delete the locked pattern.
	err = svc.Delete(ctx, userID, p.ID)
	if !errors.Is(err, domain.ErrPatternLocked) {
		t.Fatalf("expected ErrPatternLocked, got %v", err)
	}
}

func TestPatternService_Duplicate_LockedAllowed(t *testing.T) {
	svc, _, db := newTestPatternService(t)
	ctx := context.Background()

	userID := seedUserForTest(t, db, "locked-dup@example.com")
	stitchID := seedStitchForTest(t, db)

	p := &domain.Pattern{
		UserID:      userID,
		Name:        "Lockable Pattern",
		PatternType: domain.PatternTypeRound,
		InstructionGroups: []domain.InstructionGroup{
			{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: stitchID, Count: 6, RepeatCount: 1},
				}},
		},
	}
	if err := svc.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Lock the pattern directly in the DB.
	_, err := db.SqlDB.ExecContext(ctx, "UPDATE patterns SET locked = TRUE WHERE id = ?", p.ID)
	if err != nil {
		t.Fatalf("lock pattern: %v", err)
	}

	// Duplicate should succeed even when locked.
	dup, err := svc.Duplicate(ctx, userID, p.ID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if dup.Locked {
		t.Fatal("expected duplicate to be unlocked")
	}
	if len(dup.PatternStitches) != 1 {
		t.Fatalf("expected 1 pattern stitch in duplicate, got %d", len(dup.PatternStitches))
	}
}

func TestStitchCount_Simple(t *testing.T) {
	p := &domain.Pattern{
		InstructionGroups: []domain.InstructionGroup{
			{RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{Count: 6, RepeatCount: 1},
				}},
		},
	}
	if got := service.StitchCount(p); got != 6 {
		t.Fatalf("expected 6, got %d", got)
	}
}

func TestStitchCount_WithRepeats(t *testing.T) {
	p := &domain.Pattern{
		InstructionGroups: []domain.InstructionGroup{
			{RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{Count: 2, RepeatCount: 6}, // 2*6 = 12
				}},
		},
	}
	if got := service.StitchCount(p); got != 12 {
		t.Fatalf("expected 12, got %d", got)
	}
}

func TestStitchCount_WithGroupRepeats(t *testing.T) {
	p := &domain.Pattern{
		InstructionGroups: []domain.InstructionGroup{
			{RepeatCount: 3, // group repeated 3 times
				StitchEntries: []domain.StitchEntry{
					{Count: 12, RepeatCount: 1}, // 12 per iteration
				}},
		},
	}
	if got := service.StitchCount(p); got != 36 {
		t.Fatalf("expected 36, got %d", got)
	}
}

func TestStitchCount_Mixed(t *testing.T) {
	p := &domain.Pattern{
		InstructionGroups: []domain.InstructionGroup{
			{RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{Count: 6, RepeatCount: 1}, // 6
				}},
			{RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{Count: 2, RepeatCount: 6}, // 12
				}},
			{RepeatCount: 3,
				StitchEntries: []domain.StitchEntry{
					{Count: 12, RepeatCount: 1}, // 12*3 = 36
				}},
		},
	}
	// 6 + 12 + 36 = 54
	if got := service.StitchCount(p); got != 54 {
		t.Fatalf("expected 54, got %d", got)
	}
}

func TestStitchCount_MultipleEntriesInGroup(t *testing.T) {
	p := &domain.Pattern{
		InstructionGroups: []domain.InstructionGroup{
			{RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{Count: 3, RepeatCount: 1},
					{Count: 2, RepeatCount: 1},
					{Count: 1, RepeatCount: 1},
				}},
		},
	}
	if got := service.StitchCount(p); got != 6 {
		t.Fatalf("expected 6, got %d", got)
	}
}
