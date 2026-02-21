package service

import (
	"context"
	"fmt"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// PatternService handles pattern CRUD and validation.
type PatternService struct {
	patterns domain.PatternRepository
	stitches domain.StitchRepository
}

// NewPatternService creates a new PatternService.
func NewPatternService(patterns domain.PatternRepository, stitches domain.StitchRepository) *PatternService {
	return &PatternService{patterns: patterns, stitches: stitches}
}

// Create creates a new pattern with validation.
func (s *PatternService) Create(ctx context.Context, pattern *domain.Pattern) error {
	if err := s.validate(ctx, pattern); err != nil {
		return err
	}

	if err := s.patterns.Create(ctx, pattern); err != nil {
		return fmt.Errorf("create pattern: %w", err)
	}
	return nil
}

// GetByID returns a pattern by ID.
func (s *PatternService) GetByID(ctx context.Context, id int64) (*domain.Pattern, error) {
	return s.patterns.GetByID(ctx, id)
}

// ListByUser returns all patterns for a user.
func (s *PatternService) ListByUser(ctx context.Context, userID int64) ([]domain.Pattern, error) {
	return s.patterns.ListByUser(ctx, userID)
}

// Update updates a pattern with validation and ownership check.
func (s *PatternService) Update(ctx context.Context, userID int64, pattern *domain.Pattern) error {
	existing, err := s.patterns.GetByID(ctx, pattern.ID)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return domain.ErrUnauthorized
	}

	if err := s.validate(ctx, pattern); err != nil {
		return err
	}

	if err := s.patterns.Update(ctx, pattern); err != nil {
		return fmt.Errorf("update pattern: %w", err)
	}
	return nil
}

// Delete deletes a pattern with ownership check.
func (s *PatternService) Delete(ctx context.Context, userID int64, id int64) error {
	existing, err := s.patterns.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return domain.ErrUnauthorized
	}

	return s.patterns.Delete(ctx, id)
}

// Duplicate creates a copy of a pattern for a user.
func (s *PatternService) Duplicate(ctx context.Context, id int64, newUserID int64) (*domain.Pattern, error) {
	return s.patterns.Duplicate(ctx, id, newUserID)
}

// StitchCount computes the total number of individual stitches in a pattern,
// accounting for stitch counts, stitch repeats, and group repeats.
func StitchCount(pattern *domain.Pattern) int {
	total := 0
	for _, g := range pattern.InstructionGroups {
		total += GroupStitchCount(&g) * g.RepeatCount
	}
	return total
}

// GroupStitchCount computes the total stitches in a single instruction group
// (one iteration, not multiplied by group repeat).
func GroupStitchCount(g *domain.InstructionGroup) int {
	count := 0
	for _, e := range g.StitchEntries {
		count += e.Count * e.RepeatCount
	}
	return count
}

func (s *PatternService) validate(ctx context.Context, pattern *domain.Pattern) error {
	if pattern.Name == "" {
		return fmt.Errorf("%w: pattern name is required", domain.ErrInvalidInput)
	}

	if pattern.PatternType != domain.PatternTypeRound && pattern.PatternType != domain.PatternTypeRow {
		return fmt.Errorf("%w: pattern type must be 'round' or 'row'", domain.ErrInvalidInput)
	}

	validDifficulties := map[string]bool{"": true, "Beginner": true, "Intermediate": true, "Advanced": true, "Expert": true}
	if !validDifficulties[pattern.Difficulty] {
		return fmt.Errorf("%w: difficulty must be Beginner, Intermediate, Advanced, or Expert", domain.ErrInvalidInput)
	}

	if len(pattern.InstructionGroups) == 0 {
		return fmt.Errorf("%w: at least one instruction group is required", domain.ErrInvalidInput)
	}

	for i, g := range pattern.InstructionGroups {
		if g.Label == "" {
			return fmt.Errorf("%w: group %d label is required", domain.ErrInvalidInput, i+1)
		}
		if g.RepeatCount < 1 {
			return fmt.Errorf("%w: group %d repeat count must be at least 1", domain.ErrInvalidInput, i+1)
		}
		for j, e := range g.StitchEntries {
			if e.StitchID == 0 {
				return fmt.Errorf("%w: group %d entry %d has no stitch", domain.ErrInvalidInput, i+1, j+1)
			}
			// Verify stitch exists.
			if _, err := s.stitches.GetByID(ctx, e.StitchID); err != nil {
				return fmt.Errorf("%w: group %d entry %d references invalid stitch ID %d", domain.ErrInvalidInput, i+1, j+1, e.StitchID)
			}
			if e.Count < 1 {
				return fmt.Errorf("%w: group %d entry %d count must be at least 1", domain.ErrInvalidInput, i+1, j+1)
			}
			if e.RepeatCount < 1 {
				return fmt.Errorf("%w: group %d entry %d repeat count must be at least 1", domain.ErrInvalidInput, i+1, j+1)
			}
		}
	}

	return nil
}
