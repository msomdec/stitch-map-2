package sqlite_test

import (
	"context"
	"errors"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
)

func seedTestUser(t *testing.T, db *sqlite.DB) int64 {
	t.Helper()
	u := &domain.User{Email: "pattern@example.com", DisplayName: "Patt", PasswordHash: "hash"}
	if err := db.Users().Create(context.Background(), u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u.ID
}

func makeTestPattern(userID int64) *domain.Pattern {
	return &domain.Pattern{
		UserID:      userID,
		Name:        "Test Pattern",
		Description: "A test pattern",
		PatternType: domain.PatternTypeRound,
		HookSize:    "5.0mm",
		YarnWeight:  "Worsted",
		PatternStitches: []domain.PatternStitch{
			{Abbreviation: "sc", Name: "Single Crochet", Category: "basic"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				SortOrder:   0,
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: 0, Count: 6, RepeatCount: 1}, // index 0 -> "sc"
				},
			},
		},
	}
}

func TestPatternRepository_Create(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	p := makeTestPattern(userID)
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.ID == 0 {
		t.Fatal("expected pattern ID to be set")
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
	if len(p.InstructionGroups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(p.InstructionGroups))
	}
	if p.InstructionGroups[0].ID == 0 {
		t.Fatal("expected group ID to be set")
	}
	if len(p.InstructionGroups[0].StitchEntries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(p.InstructionGroups[0].StitchEntries))
	}
	if p.InstructionGroups[0].StitchEntries[0].ID == 0 {
		t.Fatal("expected entry ID to be set")
	}
	if len(p.PatternStitches) != 1 {
		t.Fatalf("expected 1 pattern stitch, got %d", len(p.PatternStitches))
	}
	if p.PatternStitches[0].ID == 0 {
		t.Fatal("expected pattern stitch ID to be set")
	}
}

func TestPatternRepository_GetByID(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	p := makeTestPattern(userID)
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if found.Name != "Test Pattern" {
		t.Fatalf("expected name 'Test Pattern', got %q", found.Name)
	}
	if found.PatternType != domain.PatternTypeRound {
		t.Fatalf("expected type 'round', got %q", found.PatternType)
	}
	if found.HookSize != "5.0mm" {
		t.Fatalf("expected hook size '5.0mm', got %q", found.HookSize)
	}
	if len(found.PatternStitches) != 1 {
		t.Fatalf("expected 1 pattern stitch, got %d", len(found.PatternStitches))
	}
	if found.PatternStitches[0].Abbreviation != "sc" {
		t.Fatalf("expected abbreviation 'sc', got %q", found.PatternStitches[0].Abbreviation)
	}
	if len(found.InstructionGroups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(found.InstructionGroups))
	}
	if found.InstructionGroups[0].Label != "Round 1" {
		t.Fatalf("expected group label 'Round 1', got %q", found.InstructionGroups[0].Label)
	}
	if len(found.InstructionGroups[0].StitchEntries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(found.InstructionGroups[0].StitchEntries))
	}
	if found.InstructionGroups[0].StitchEntries[0].Count != 6 {
		t.Fatalf("expected count 6, got %d", found.InstructionGroups[0].StitchEntries[0].Count)
	}
	// Entry should reference the pattern stitch.
	if found.InstructionGroups[0].StitchEntries[0].PatternStitchID != found.PatternStitches[0].ID {
		t.Fatalf("expected entry to reference pattern stitch ID %d, got %d",
			found.PatternStitches[0].ID, found.InstructionGroups[0].StitchEntries[0].PatternStitchID)
	}
}

func TestPatternRepository_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 99999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPatternRepository_ListByUser(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	p1 := makeTestPattern(userID)
	p1.Name = "Pattern 1"
	if err := repo.Create(ctx, p1); err != nil {
		t.Fatalf("Create p1: %v", err)
	}

	p2 := makeTestPattern(userID)
	p2.Name = "Pattern 2"
	if err := repo.Create(ctx, p2); err != nil {
		t.Fatalf("Create p2: %v", err)
	}

	patterns, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
}

func TestPatternRepository_ListByUser_Empty(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	patterns, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if patterns != nil && len(patterns) != 0 {
		t.Fatalf("expected 0 patterns, got %d", len(patterns))
	}
}

func TestPatternRepository_Update(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	p := makeTestPattern(userID)
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update metadata and groups.
	p.Name = "Updated Pattern"
	p.Description = "Updated description"
	p.PatternStitches = []domain.PatternStitch{
		{Abbreviation: "sc", Name: "Single Crochet", Category: "basic"},
	}
	p.InstructionGroups = []domain.InstructionGroup{
		{SortOrder: 0, Label: "Round 1", RepeatCount: 1,
			StitchEntries: []domain.StitchEntry{
				{SortOrder: 0, PatternStitchID: 0, Count: 6, RepeatCount: 1},
			}},
		{SortOrder: 1, Label: "Round 2", RepeatCount: 1,
			StitchEntries: []domain.StitchEntry{
				{SortOrder: 0, PatternStitchID: 0, Count: 12, RepeatCount: 1},
			}},
	}

	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	found, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if found.Name != "Updated Pattern" {
		t.Fatalf("expected name 'Updated Pattern', got %q", found.Name)
	}
	if len(found.InstructionGroups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(found.InstructionGroups))
	}
	if found.InstructionGroups[1].Label != "Round 2" {
		t.Fatalf("expected second group label 'Round 2', got %q", found.InstructionGroups[1].Label)
	}
}

func TestPatternRepository_Update_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	p := &domain.Pattern{ID: 99999, Name: "Nonexistent"}
	err := repo.Update(ctx, p)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPatternRepository_Delete(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	p := makeTestPattern(userID)
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetByID(ctx, p.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestPatternRepository_Delete_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	err := repo.Delete(ctx, 99999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPatternRepository_Delete_Cascades(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	p := makeTestPattern(userID)
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Delete the pattern and verify groups/entries are gone.
	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var groupCount int
	db.SqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM instruction_groups WHERE pattern_id = ?", p.ID).Scan(&groupCount)
	if groupCount != 0 {
		t.Fatalf("expected 0 groups after delete, got %d", groupCount)
	}

	var psCount int
	db.SqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM pattern_stitches WHERE pattern_id = ?", p.ID).Scan(&psCount)
	if psCount != 0 {
		t.Fatalf("expected 0 pattern stitches after delete, got %d", psCount)
	}
}

func TestPatternRepository_Duplicate(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	original := makeTestPattern(userID)
	original.Name = "Original"
	original.Difficulty = "Beginner"
	if err := repo.Create(ctx, original); err != nil {
		t.Fatalf("Create: %v", err)
	}

	dup, err := repo.Duplicate(ctx, original.ID, userID)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}

	if dup.ID == original.ID {
		t.Fatal("duplicate should have different ID")
	}
	if dup.Name != "Original (Copy)" {
		t.Fatalf("expected name 'Original (Copy)', got %q", dup.Name)
	}
	if dup.Difficulty != "Beginner" {
		t.Fatalf("expected difficulty to be copied, got %q", dup.Difficulty)
	}
	if len(dup.InstructionGroups) != len(original.InstructionGroups) {
		t.Fatalf("expected same number of groups")
	}
	if len(dup.PatternStitches) != len(original.PatternStitches) {
		t.Fatalf("expected same number of pattern stitches")
	}
	// Pattern stitches should have different IDs.
	if dup.PatternStitches[0].ID == original.PatternStitches[0].ID {
		t.Fatal("duplicate pattern stitch should have different ID")
	}

	// Verify independence - deleting duplicate doesn't affect original.
	if err := repo.Delete(ctx, dup.ID); err != nil {
		t.Fatalf("Delete duplicate: %v", err)
	}
	_, err = repo.GetByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("original should still exist: %v", err)
	}
}

func TestPatternRepository_MultipleGroupsAndEntries(t *testing.T) {
	db := newTestDB(t)
	repo := db.Patterns()
	ctx := context.Background()

	userID := seedTestUser(t, db)

	expectedCount := 12
	p := &domain.Pattern{
		UserID:      userID,
		Name:        "Complex Pattern",
		PatternType: domain.PatternTypeRound,
		PatternStitches: []domain.PatternStitch{
			{Abbreviation: "sc", Name: "Single Crochet", Category: "basic"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				SortOrder: 0, Label: "Round 1", RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: 0, Count: 6, RepeatCount: 1},
				},
			},
			{
				SortOrder: 1, Label: "Round 2", RepeatCount: 1,
				ExpectedCount: &expectedCount,
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: 0, Count: 2, RepeatCount: 6, IntoStitch: "into each st"},
				},
			},
			{
				SortOrder: 2, Label: "Rounds 3-5", RepeatCount: 3,
				Notes: "in each st around",
				StitchEntries: []domain.StitchEntry{
					{SortOrder: 0, PatternStitchID: 0, Count: 12, RepeatCount: 1},
				},
			},
		},
	}

	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if len(found.InstructionGroups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(found.InstructionGroups))
	}

	g2 := found.InstructionGroups[1]
	if g2.ExpectedCount == nil || *g2.ExpectedCount != 12 {
		t.Fatal("expected Round 2 expected_count = 12")
	}
	if g2.StitchEntries[0].IntoStitch != "into each st" {
		t.Fatalf("expected into_stitch 'into each st', got %q", g2.StitchEntries[0].IntoStitch)
	}

	g3 := found.InstructionGroups[2]
	if g3.RepeatCount != 3 {
		t.Fatalf("expected repeat count 3, got %d", g3.RepeatCount)
	}
	if g3.Notes != "in each st around" {
		t.Fatalf("expected group notes 'in each st around', got %q", g3.Notes)
	}
}
