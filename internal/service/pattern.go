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
	if err := s.validate(pattern); err != nil {
		return err
	}

	if err := s.resolvePatternStitches(ctx, pattern); err != nil {
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

// GetNamesByIDs returns a map of pattern ID to name for the given IDs.
func (s *PatternService) GetNamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	return s.patterns.GetNamesByIDs(ctx, ids)
}

// ListByUser returns all patterns for a user.
func (s *PatternService) ListByUser(ctx context.Context, userID int64) ([]domain.Pattern, error) {
	return s.patterns.ListByUser(ctx, userID)
}

// ListSharedWithUser returns all patterns shared with a user.
func (s *PatternService) ListSharedWithUser(ctx context.Context, userID int64) ([]domain.Pattern, error) {
	return s.patterns.ListSharedWithUser(ctx, userID)
}

// ListSummaryByUser returns lightweight pattern summaries for a user's own patterns.
func (s *PatternService) ListSummaryByUser(ctx context.Context, userID int64) ([]domain.PatternSummary, error) {
	return s.patterns.ListSummaryByUser(ctx, userID)
}

// ListSummarySharedWithUser returns lightweight pattern summaries for patterns shared with a user.
func (s *PatternService) ListSummarySharedWithUser(ctx context.Context, userID int64) ([]domain.PatternSummary, error) {
	return s.patterns.ListSummarySharedWithUser(ctx, userID)
}

// SearchSummaryByUser returns filtered and sorted pattern summaries for a user's own patterns.
func (s *PatternService) SearchSummaryByUser(ctx context.Context, userID int64, filter domain.PatternFilter) ([]domain.PatternSummary, error) {
	return s.patterns.SearchSummaryByUser(ctx, userID, filter)
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
	if existing.SharedFromUserID != nil {
		return domain.ErrPatternLocked
	}
	if existing.Locked {
		return domain.ErrPatternLocked
	}

	if err := s.validate(pattern); err != nil {
		return err
	}

	if err := s.resolvePatternStitches(ctx, pattern); err != nil {
		return err
	}

	if err := s.patterns.Update(ctx, pattern); err != nil {
		return fmt.Errorf("update pattern: %w", err)
	}
	return nil
}

// Delete deletes a pattern with ownership check.
// Received patterns (SharedFromUserID != nil) cannot be deleted.
func (s *PatternService) Delete(ctx context.Context, userID int64, id int64) error {
	existing, err := s.patterns.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return domain.ErrUnauthorized
	}
	if existing.SharedFromUserID != nil {
		return domain.ErrPatternLocked
	}
	if existing.Locked {
		return domain.ErrPatternLocked
	}

	return s.patterns.Delete(ctx, id)
}

// Duplicate creates a copy of a pattern for a user.
// Received patterns (SharedFromUserID != nil) cannot be duplicated.
func (s *PatternService) Duplicate(ctx context.Context, userID int64, id int64, newUserID int64) (*domain.Pattern, error) {
	existing, err := s.patterns.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.UserID != userID {
		return nil, domain.ErrUnauthorized
	}
	if existing.SharedFromUserID != nil {
		return nil, fmt.Errorf("%w: cannot duplicate a received pattern", domain.ErrPatternLocked)
	}
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

// resolvePatternStitches collects unique library stitch IDs from entries,
// fetches them, builds PatternStitch copies, and remaps entry references
// from library stitch IDs to PatternStitch slice indices.
func (s *PatternService) resolvePatternStitches(ctx context.Context, pattern *domain.Pattern) error {
	// Collect unique library stitch IDs from all entries.
	// At this point, entry.PatternStitchID temporarily holds library stitch IDs from the form.
	seen := make(map[int64]int) // libraryStitchID -> index in PatternStitches
	var patternStitches []domain.PatternStitch

	for _, g := range pattern.InstructionGroups {
		for _, e := range g.StitchEntries {
			libID := e.PatternStitchID
			if _, ok := seen[libID]; ok {
				continue
			}

			stitch, err := s.stitches.GetByID(ctx, libID)
			if err != nil {
				return fmt.Errorf("%w: references invalid stitch ID %d", domain.ErrInvalidInput, libID)
			}

			idx := len(patternStitches)
			seen[libID] = idx
			patternStitches = append(patternStitches, domain.PatternStitch{
				Abbreviation:    stitch.Abbreviation,
				Name:            stitch.Name,
				Description:     stitch.Description,
				Category:        stitch.Category,
				LibraryStitchID: &libID,
			})
		}
	}

	// Remap all entry.PatternStitchID from library stitch IDs to PatternStitch slice indices.
	for i := range pattern.InstructionGroups {
		for j := range pattern.InstructionGroups[i].StitchEntries {
			e := &pattern.InstructionGroups[i].StitchEntries[j]
			idx := seen[e.PatternStitchID]
			e.PatternStitchID = int64(idx)
		}
	}

	pattern.PatternStitches = patternStitches
	return nil
}

func (s *PatternService) validate(pattern *domain.Pattern) error {
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
			if e.PatternStitchID == 0 {
				return fmt.Errorf("%w: group %d entry %d has no stitch", domain.ErrInvalidInput, i+1, j+1)
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
