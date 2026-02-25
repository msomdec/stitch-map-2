package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// ShareService handles pattern sharing operations.
type ShareService struct {
	shares   domain.PatternShareRepository
	patterns domain.PatternRepository
	users    domain.UserRepository
}

// NewShareService creates a new ShareService.
func NewShareService(shares domain.PatternShareRepository, patterns domain.PatternRepository, users domain.UserRepository) *ShareService {
	return &ShareService{shares: shares, patterns: patterns, users: users}
}

// CreateGlobalShare creates a global share link for a pattern.
// If one already exists, returns it (idempotent).
func (s *ShareService) CreateGlobalShare(ctx context.Context, userID, patternID int64) (*domain.PatternShare, error) {
	pattern, err := s.patterns.GetByID(ctx, patternID)
	if err != nil {
		return nil, err
	}
	if pattern.UserID != userID {
		return nil, domain.ErrUnauthorized
	}
	if pattern.SharedFromUserID != nil {
		return nil, fmt.Errorf("%w: cannot share a received pattern", domain.ErrInvalidInput)
	}

	// Check for existing global share.
	existing, err := s.shares.ListByPattern(ctx, patternID)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	for _, share := range existing {
		if share.ShareType == domain.ShareTypeGlobal {
			return &share, nil
		}
	}

	token, err := generateShareToken()
	if err != nil {
		return nil, err
	}

	share := &domain.PatternShare{
		PatternID: patternID,
		Token:     token,
		ShareType: domain.ShareTypeGlobal,
	}
	if err := s.shares.Create(ctx, share); err != nil {
		return nil, fmt.Errorf("create share: %w", err)
	}
	return share, nil
}

// CreateEmailShare creates an email-bound share link for a pattern.
// If one already exists for the same email, returns it (idempotent).
func (s *ShareService) CreateEmailShare(ctx context.Context, userID, patternID int64, recipientEmail string) (*domain.PatternShare, error) {
	if _, err := mail.ParseAddress(recipientEmail); err != nil {
		return nil, fmt.Errorf("%w: invalid email address", domain.ErrInvalidInput)
	}

	pattern, err := s.patterns.GetByID(ctx, patternID)
	if err != nil {
		return nil, err
	}
	if pattern.UserID != userID {
		return nil, domain.ErrUnauthorized
	}
	if pattern.SharedFromUserID != nil {
		return nil, fmt.Errorf("%w: cannot share a received pattern", domain.ErrInvalidInput)
	}

	// Reject self-share.
	owner, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get owner: %w", err)
	}
	if owner.Email == recipientEmail {
		return nil, fmt.Errorf("%w: cannot share with yourself", domain.ErrInvalidInput)
	}

	// Check for existing email share with this recipient.
	existing, err := s.shares.ListByPattern(ctx, patternID)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	for _, share := range existing {
		if share.ShareType == domain.ShareTypeEmail && share.RecipientEmail == recipientEmail {
			return &share, nil
		}
	}

	token, err := generateShareToken()
	if err != nil {
		return nil, err
	}

	share := &domain.PatternShare{
		PatternID:      patternID,
		Token:          token,
		ShareType:      domain.ShareTypeEmail,
		RecipientEmail: recipientEmail,
	}
	if err := s.shares.Create(ctx, share); err != nil {
		return nil, fmt.Errorf("create email share: %w", err)
	}
	return share, nil
}

// RevokeShareForPattern revokes a single share after verifying pattern ownership.
func (s *ShareService) RevokeShareForPattern(ctx context.Context, userID, patternID, shareID int64) error {
	pattern, err := s.patterns.GetByID(ctx, patternID)
	if err != nil {
		return err
	}
	if pattern.UserID != userID {
		return domain.ErrUnauthorized
	}

	return s.shares.Delete(ctx, shareID)
}

// RevokeAllShares revokes all shares for a pattern.
func (s *ShareService) RevokeAllShares(ctx context.Context, userID, patternID int64) error {
	pattern, err := s.patterns.GetByID(ctx, patternID)
	if err != nil {
		return err
	}
	if pattern.UserID != userID {
		return domain.ErrUnauthorized
	}

	return s.shares.DeleteAllByPattern(ctx, patternID)
}

// GetPatternByShareToken loads a pattern by share token, enforcing access rules.
func (s *ShareService) GetPatternByShareToken(ctx context.Context, viewerUserID int64, token string) (*domain.Pattern, error) {
	share, err := s.shares.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if share.ShareType == domain.ShareTypeEmail {
		viewer, err := s.users.GetByID(ctx, viewerUserID)
		if err != nil {
			return nil, fmt.Errorf("get viewer: %w", err)
		}
		if viewer.Email != share.RecipientEmail {
			return nil, domain.ErrUnauthorized
		}
	}

	return s.patterns.GetByID(ctx, share.PatternID)
}

// SaveSharedPattern saves a shared pattern to the viewer's library as a locked snapshot.
func (s *ShareService) SaveSharedPattern(ctx context.Context, viewerUserID int64, token string) (*domain.Pattern, error) {
	share, err := s.shares.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Enforce email-bound access.
	if share.ShareType == domain.ShareTypeEmail {
		viewer, err := s.users.GetByID(ctx, viewerUserID)
		if err != nil {
			return nil, fmt.Errorf("get viewer: %w", err)
		}
		if viewer.Email != share.RecipientEmail {
			return nil, domain.ErrUnauthorized
		}
	}

	pattern, err := s.patterns.GetByID(ctx, share.PatternID)
	if err != nil {
		return nil, err
	}

	// Don't allow saving your own pattern.
	if pattern.UserID == viewerUserID {
		return nil, fmt.Errorf("%w: cannot save your own pattern", domain.ErrInvalidInput)
	}

	// Check for duplicate save — has the viewer already saved this pattern from the same owner?
	sharedPatterns, err := s.patterns.ListSharedWithUser(ctx, viewerUserID)
	if err != nil {
		return nil, fmt.Errorf("list shared patterns: %w", err)
	}
	for _, sp := range sharedPatterns {
		if sp.SharedFromUserID != nil && *sp.SharedFromUserID == pattern.UserID && sp.Name == pattern.Name {
			return nil, domain.ErrAlreadySaved
		}
	}

	// Get owner display name for denormalized storage.
	owner, err := s.users.GetByID(ctx, pattern.UserID)
	if err != nil {
		return nil, fmt.Errorf("get owner: %w", err)
	}

	return s.patterns.DuplicateAsShared(ctx, pattern.ID, viewerUserID, pattern.UserID, owner.DisplayName)
}

// ListSharesForPattern returns all active shares for a pattern (owner only).
func (s *ShareService) ListSharesForPattern(ctx context.Context, userID, patternID int64) ([]domain.PatternShare, error) {
	pattern, err := s.patterns.GetByID(ctx, patternID)
	if err != nil {
		return nil, err
	}
	if pattern.UserID != userID {
		return nil, domain.ErrUnauthorized
	}

	return s.shares.ListByPattern(ctx, patternID)
}

// HasSharesByPatternIDs returns which pattern IDs have active shares.
func (s *ShareService) HasSharesByPatternIDs(ctx context.Context, patternIDs []int64) (map[int64]bool, error) {
	return s.shares.HasSharesByPatternIDs(ctx, patternIDs)
}

// GetShareByToken returns a share by its token.
func (s *ShareService) GetShareByToken(ctx context.Context, token string) (*domain.PatternShare, error) {
	return s.shares.GetByToken(ctx, token)
}

func generateShareToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate share token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
