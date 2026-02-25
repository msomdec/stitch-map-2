package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
	"github.com/msomdec/stitch-map-2/internal/service"
)

func newTestShareService(t *testing.T) (*service.ShareService, *service.PatternService, *service.StitchService, *sqlite.DB) {
	t.Helper()
	_, db := newTestAuthService(t)
	shareRepo := db.Shares()
	patternRepo := db.Patterns()
	stitchRepo := db.Stitches()
	userRepo := db.Users()
	return service.NewShareService(shareRepo, patternRepo, userRepo),
		service.NewPatternService(patternRepo, stitchRepo),
		service.NewStitchService(stitchRepo),
		db
}

func createTestPattern(t *testing.T, patternSvc *service.PatternService, db *sqlite.DB, userID int64) *domain.Pattern {
	t.Helper()
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
	if err := patternSvc.Create(context.Background(), p); err != nil {
		t.Fatalf("create test pattern: %v", err)
	}
	return p
}

func TestShareService_CreateGlobalShare_Success(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	userID := seedUserForTest(t, db, "owner@example.com")
	p := createTestPattern(t, patternSvc, db, userID)

	share, err := shareSvc.CreateGlobalShare(ctx, userID, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}
	if share.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if share.ShareType != domain.ShareTypeGlobal {
		t.Fatalf("expected global share type, got %s", share.ShareType)
	}
	if len(share.Token) != 64 {
		t.Fatalf("expected 64-char token, got %d", len(share.Token))
	}
}

func TestShareService_CreateGlobalShare_Idempotent(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	userID := seedUserForTest(t, db, "idem@example.com")
	p := createTestPattern(t, patternSvc, db, userID)

	share1, err := shareSvc.CreateGlobalShare(ctx, userID, p.ID)
	if err != nil {
		t.Fatalf("first CreateGlobalShare: %v", err)
	}

	share2, err := shareSvc.CreateGlobalShare(ctx, userID, p.ID)
	if err != nil {
		t.Fatalf("second CreateGlobalShare: %v", err)
	}

	if share1.Token != share2.Token {
		t.Fatalf("expected same token, got %s and %s", share1.Token, share2.Token)
	}
}

func TestShareService_CreateGlobalShare_NonOwner(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "owner2@example.com")
	other := seedUserForTest(t, db, "other2@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	_, err := shareSvc.CreateGlobalShare(ctx, other, p.ID)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestShareService_CreateGlobalShare_ReshareReceived(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "reshare-owner@example.com")
	recipient := seedUserForTest(t, db, "reshare-recip@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	// Create a share and save it.
	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}
	saved, err := shareSvc.SaveSharedPattern(ctx, recipient, share.Token)
	if err != nil {
		t.Fatalf("SaveSharedPattern: %v", err)
	}

	// Recipient should not be able to reshare.
	_, err = shareSvc.CreateGlobalShare(ctx, recipient, saved.ID)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for reshare, got %v", err)
	}
}

func TestShareService_CreateEmailShare_Success(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	userID := seedUserForTest(t, db, "emailowner@example.com")
	p := createTestPattern(t, patternSvc, db, userID)

	share, err := shareSvc.CreateEmailShare(ctx, userID, p.ID, "friend@example.com")
	if err != nil {
		t.Fatalf("CreateEmailShare: %v", err)
	}
	if share.ShareType != domain.ShareTypeEmail {
		t.Fatalf("expected email share type, got %s", share.ShareType)
	}
	if share.RecipientEmail != "friend@example.com" {
		t.Fatalf("expected recipient email friend@example.com, got %s", share.RecipientEmail)
	}
}

func TestShareService_CreateEmailShare_Idempotent(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	userID := seedUserForTest(t, db, "emailidem@example.com")
	p := createTestPattern(t, patternSvc, db, userID)

	share1, err := shareSvc.CreateEmailShare(ctx, userID, p.ID, "friend@example.com")
	if err != nil {
		t.Fatalf("first CreateEmailShare: %v", err)
	}

	share2, err := shareSvc.CreateEmailShare(ctx, userID, p.ID, "friend@example.com")
	if err != nil {
		t.Fatalf("second CreateEmailShare: %v", err)
	}

	if share1.Token != share2.Token {
		t.Fatalf("expected same token, got %s and %s", share1.Token, share2.Token)
	}
}

func TestShareService_CreateEmailShare_InvalidEmail(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	userID := seedUserForTest(t, db, "invalidemail@example.com")
	p := createTestPattern(t, patternSvc, db, userID)

	_, err := shareSvc.CreateEmailShare(ctx, userID, p.ID, "not-an-email")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestShareService_CreateEmailShare_SelfShare(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	userID := seedUserForTest(t, db, "self@example.com")
	p := createTestPattern(t, patternSvc, db, userID)

	_, err := shareSvc.CreateEmailShare(ctx, userID, p.ID, "self@example.com")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for self-share, got %v", err)
	}
}

func TestShareService_GetPatternByShareToken_Global(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "viewowner@example.com")
	viewer := seedUserForTest(t, db, "viewer@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}

	pattern, err := shareSvc.GetPatternByShareToken(ctx, viewer, share.Token)
	if err != nil {
		t.Fatalf("GetPatternByShareToken: %v", err)
	}
	if pattern.ID != p.ID {
		t.Fatalf("expected pattern ID %d, got %d", p.ID, pattern.ID)
	}
}

func TestShareService_GetPatternByShareToken_InvalidToken(t *testing.T) {
	shareSvc, _, _, db := newTestShareService(t)
	ctx := context.Background()
	viewer := seedUserForTest(t, db, "viewerbad@example.com")

	_, err := shareSvc.GetPatternByShareToken(ctx, viewer, "nonexistent-token")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestShareService_GetPatternByShareToken_EmailMatch(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "emailmatchowner@example.com")
	recipient := seedUserForTest(t, db, "match@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateEmailShare(ctx, owner, p.ID, "match@example.com")
	if err != nil {
		t.Fatalf("CreateEmailShare: %v", err)
	}

	// Matching email should succeed.
	pattern, err := shareSvc.GetPatternByShareToken(ctx, recipient, share.Token)
	if err != nil {
		t.Fatalf("GetPatternByShareToken: %v", err)
	}
	if pattern.ID != p.ID {
		t.Fatalf("expected pattern ID %d, got %d", p.ID, pattern.ID)
	}
}

func TestShareService_GetPatternByShareToken_EmailMismatch(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "mismatchowner@example.com")
	wrong := seedUserForTest(t, db, "wrong@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateEmailShare(ctx, owner, p.ID, "correct@example.com")
	if err != nil {
		t.Fatalf("CreateEmailShare: %v", err)
	}

	// Non-matching email should fail.
	_, err = shareSvc.GetPatternByShareToken(ctx, wrong, share.Token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for email mismatch, got %v", err)
	}
}

func TestShareService_SaveSharedPattern_Success(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "saveowner@example.com")
	viewer := seedUserForTest(t, db, "saveviewer@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}

	saved, err := shareSvc.SaveSharedPattern(ctx, viewer, share.Token)
	if err != nil {
		t.Fatalf("SaveSharedPattern: %v", err)
	}

	if saved.UserID != viewer {
		t.Fatalf("expected user ID %d, got %d", viewer, saved.UserID)
	}
	if !saved.Locked {
		t.Fatal("expected saved pattern to be locked")
	}
	if saved.SharedFromUserID == nil {
		t.Fatal("expected SharedFromUserID to be set")
	}
	if *saved.SharedFromUserID != owner {
		t.Fatalf("expected SharedFromUserID %d, got %d", owner, *saved.SharedFromUserID)
	}
	if saved.SharedFromName == "" {
		t.Fatal("expected SharedFromName to be set")
	}
	if saved.Name != p.Name {
		t.Fatalf("expected name %q, got %q", p.Name, saved.Name)
	}
	if len(saved.PatternStitches) != len(p.PatternStitches) {
		t.Fatalf("expected %d pattern stitches, got %d", len(p.PatternStitches), len(saved.PatternStitches))
	}
}

func TestShareService_SaveSharedPattern_DuplicatePrevention(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "dupowner@example.com")
	viewer := seedUserForTest(t, db, "dupviewer@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}

	// First save should succeed.
	_, err = shareSvc.SaveSharedPattern(ctx, viewer, share.Token)
	if err != nil {
		t.Fatalf("first SaveSharedPattern: %v", err)
	}

	// Second save should fail.
	_, err = shareSvc.SaveSharedPattern(ctx, viewer, share.Token)
	if !errors.Is(err, domain.ErrAlreadySaved) {
		t.Fatalf("expected ErrAlreadySaved, got %v", err)
	}
}

func TestShareService_SaveSharedPattern_CannotSaveOwn(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "ownself@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}

	_, err = shareSvc.SaveSharedPattern(ctx, owner, share.Token)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for saving own pattern, got %v", err)
	}
}

func TestShareService_RevokeShare(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "revokeowner@example.com")
	viewer := seedUserForTest(t, db, "revokeviewer@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}

	// Revoke the share.
	if err := shareSvc.RevokeShareForPattern(ctx, owner, p.ID, share.ID); err != nil {
		t.Fatalf("RevokeShareForPattern: %v", err)
	}

	// Viewing should now fail.
	_, err = shareSvc.GetPatternByShareToken(ctx, viewer, share.Token)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after revoke, got %v", err)
	}
}

func TestShareService_RevokeAllShares(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "revokeallowner@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	_, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}
	_, err = shareSvc.CreateEmailShare(ctx, owner, p.ID, "a@b.com")
	if err != nil {
		t.Fatalf("CreateEmailShare: %v", err)
	}

	// Should have 2 shares.
	shares, err := shareSvc.ListSharesForPattern(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("ListSharesForPattern: %v", err)
	}
	if len(shares) != 2 {
		t.Fatalf("expected 2 shares, got %d", len(shares))
	}

	// Revoke all.
	if err := shareSvc.RevokeAllShares(ctx, owner, p.ID); err != nil {
		t.Fatalf("RevokeAllShares: %v", err)
	}

	// Should have 0 shares.
	shares, err = shareSvc.ListSharesForPattern(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("ListSharesForPattern after revoke: %v", err)
	}
	if len(shares) != 0 {
		t.Fatalf("expected 0 shares after revoke all, got %d", len(shares))
	}
}

func TestShareService_ReceivedPatternImmutability(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "immut-owner@example.com")
	recipient := seedUserForTest(t, db, "immut-recip@example.com")
	p := createTestPattern(t, patternSvc, db, owner)

	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}

	saved, err := shareSvc.SaveSharedPattern(ctx, recipient, share.Token)
	if err != nil {
		t.Fatalf("SaveSharedPattern: %v", err)
	}

	// Edit should fail.
	saved.Name = "Hacked"
	if err := patternSvc.Update(ctx, recipient, saved); !errors.Is(err, domain.ErrPatternLocked) {
		t.Fatalf("expected ErrPatternLocked for edit, got %v", err)
	}

	// Delete should fail.
	if err := patternSvc.Delete(ctx, recipient, saved.ID); !errors.Is(err, domain.ErrPatternLocked) {
		t.Fatalf("expected ErrPatternLocked for delete, got %v", err)
	}

	// Duplicate should fail.
	_, err = patternSvc.Duplicate(ctx, recipient, saved.ID, recipient)
	if !errors.Is(err, domain.ErrPatternLocked) {
		t.Fatalf("expected ErrPatternLocked for duplicate, got %v", err)
	}

	// Reshare should fail.
	_, err = shareSvc.CreateGlobalShare(ctx, recipient, saved.ID)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for reshare, got %v", err)
	}
}

func TestShareService_HasSharesByPatternIDs(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "hasshare@example.com")
	p1 := createTestPattern(t, patternSvc, db, owner)
	p2 := createTestPattern(t, patternSvc, db, owner)

	// Share only p1.
	_, err := shareSvc.CreateGlobalShare(ctx, owner, p1.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}

	result, err := shareSvc.HasSharesByPatternIDs(ctx, []int64{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("HasSharesByPatternIDs: %v", err)
	}

	if !result[p1.ID] {
		t.Fatal("expected p1 to have shares")
	}
	if result[p2.ID] {
		t.Fatal("expected p2 to not have shares")
	}
}

func TestShareService_PatternListSections(t *testing.T) {
	shareSvc, patternSvc, _, db := newTestShareService(t)
	ctx := context.Background()
	owner := seedUserForTest(t, db, "listowner@example.com")
	recipient := seedUserForTest(t, db, "listrecip@example.com")

	p := createTestPattern(t, patternSvc, db, owner)
	recipPattern := createTestPattern(t, patternSvc, db, recipient)
	_ = recipPattern

	// Share owner's pattern and save it as recipient.
	share, err := shareSvc.CreateGlobalShare(ctx, owner, p.ID)
	if err != nil {
		t.Fatalf("CreateGlobalShare: %v", err)
	}
	_, err = shareSvc.SaveSharedPattern(ctx, recipient, share.Token)
	if err != nil {
		t.Fatalf("SaveSharedPattern: %v", err)
	}

	// ListByUser should show only user-authored patterns.
	myPatterns, err := patternSvc.ListByUser(ctx, recipient)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	for _, mp := range myPatterns {
		if mp.SharedFromUserID != nil {
			t.Fatalf("ListByUser returned a received pattern: %d", mp.ID)
		}
	}

	// ListSharedWithUser should show only received patterns.
	sharedPatterns, err := patternSvc.ListSharedWithUser(ctx, recipient)
	if err != nil {
		t.Fatalf("ListSharedWithUser: %v", err)
	}
	if len(sharedPatterns) != 1 {
		t.Fatalf("expected 1 shared pattern, got %d", len(sharedPatterns))
	}
	if sharedPatterns[0].SharedFromUserID == nil {
		t.Fatal("expected SharedFromUserID to be set on shared pattern")
	}
}
