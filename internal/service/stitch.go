package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// StitchService handles stitch library operations.
type StitchService struct {
	stitches domain.StitchRepository
}

// NewStitchService creates a new StitchService.
func NewStitchService(stitches domain.StitchRepository) *StitchService {
	return &StitchService{stitches: stitches}
}

// ListPredefined returns all predefined stitches.
func (s *StitchService) ListPredefined(ctx context.Context) ([]domain.Stitch, error) {
	return s.stitches.ListPredefined(ctx)
}

// ListByUser returns a user's custom stitches.
func (s *StitchService) ListByUser(ctx context.Context, userID int64) ([]domain.Stitch, error) {
	return s.stitches.ListByUser(ctx, userID)
}

// ListAll returns predefined stitches combined with a user's custom stitches.
func (s *StitchService) ListAll(ctx context.Context, userID int64) ([]domain.Stitch, error) {
	predefined, err := s.stitches.ListPredefined(ctx)
	if err != nil {
		return nil, fmt.Errorf("list predefined: %w", err)
	}

	custom, err := s.stitches.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user stitches: %w", err)
	}

	return append(predefined, custom...), nil
}

// GetByID returns a stitch by its ID.
func (s *StitchService) GetByID(ctx context.Context, id int64) (*domain.Stitch, error) {
	return s.stitches.GetByID(ctx, id)
}

// CreateCustom creates a new custom stitch for a user.
// It rejects abbreviations that conflict with predefined stitches.
func (s *StitchService) CreateCustom(ctx context.Context, userID int64, abbreviation, name, description, category string) (*domain.Stitch, error) {
	if abbreviation == "" || name == "" {
		return nil, fmt.Errorf("%w: abbreviation and name are required", domain.ErrInvalidInput)
	}
	if len(abbreviation) > 20 {
		return nil, fmt.Errorf("%w: abbreviation must be 20 characters or fewer", domain.ErrInvalidInput)
	}
	if len(name) > 100 {
		return nil, fmt.Errorf("%w: name must be 100 characters or fewer", domain.ErrInvalidInput)
	}
	if len(description) > 1000 {
		return nil, fmt.Errorf("%w: description must be 1000 characters or fewer", domain.ErrInvalidInput)
	}
	if len(category) > 50 {
		return nil, fmt.Errorf("%w: category must be 50 characters or fewer", domain.ErrInvalidInput)
	}

	// Check if abbreviation conflicts with a predefined stitch.
	_, err := s.stitches.GetByAbbreviation(ctx, abbreviation, nil)
	if err == nil {
		return nil, fmt.Errorf("%w: '%s' is a predefined stitch abbreviation", domain.ErrReservedAbbreviation, abbreviation)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("check predefined abbreviation: %w", err)
	}

	if category == "" {
		category = "custom"
	}

	stitch := &domain.Stitch{
		Abbreviation: abbreviation,
		Name:         name,
		Description:  description,
		Category:     category,
		IsCustom:     true,
		UserID:       &userID,
	}

	if err := s.stitches.Create(ctx, stitch); err != nil {
		return nil, fmt.Errorf("create custom stitch: %w", err)
	}

	return stitch, nil
}

// UpdateCustom updates an existing custom stitch. Only the owner can update it.
func (s *StitchService) UpdateCustom(ctx context.Context, userID int64, id int64, abbreviation, name, description, category string) (*domain.Stitch, error) {
	stitch, err := s.stitches.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !stitch.IsCustom || stitch.UserID == nil || *stitch.UserID != userID {
		return nil, domain.ErrUnauthorized
	}

	if abbreviation == "" || name == "" {
		return nil, fmt.Errorf("%w: abbreviation and name are required", domain.ErrInvalidInput)
	}
	if len(abbreviation) > 20 {
		return nil, fmt.Errorf("%w: abbreviation must be 20 characters or fewer", domain.ErrInvalidInput)
	}
	if len(name) > 100 {
		return nil, fmt.Errorf("%w: name must be 100 characters or fewer", domain.ErrInvalidInput)
	}
	if len(description) > 1000 {
		return nil, fmt.Errorf("%w: description must be 1000 characters or fewer", domain.ErrInvalidInput)
	}
	if len(category) > 50 {
		return nil, fmt.Errorf("%w: category must be 50 characters or fewer", domain.ErrInvalidInput)
	}

	// If abbreviation changed, check it doesn't conflict with predefined.
	if abbreviation != stitch.Abbreviation {
		_, err := s.stitches.GetByAbbreviation(ctx, abbreviation, nil)
		if err == nil {
			return nil, fmt.Errorf("%w: '%s' is a predefined stitch abbreviation", domain.ErrReservedAbbreviation, abbreviation)
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("check predefined abbreviation: %w", err)
		}
	}

	stitch.Abbreviation = abbreviation
	stitch.Name = name
	stitch.Description = description
	if category != "" {
		stitch.Category = category
	}

	if err := s.stitches.Update(ctx, stitch); err != nil {
		return nil, fmt.Errorf("update custom stitch: %w", err)
	}

	return stitch, nil
}

// DeleteCustom deletes a custom stitch. Only the owner can delete it.
func (s *StitchService) DeleteCustom(ctx context.Context, userID int64, id int64) error {
	stitch, err := s.stitches.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !stitch.IsCustom || stitch.UserID == nil || *stitch.UserID != userID {
		return domain.ErrUnauthorized
	}

	return s.stitches.Delete(ctx, id)
}

// SeedPredefined inserts all predefined stitches. It is idempotent — existing
// stitches are skipped based on abbreviation with NULL user_id.
func (s *StitchService) SeedPredefined(ctx context.Context) error {
	for _, st := range predefinedStitches {
		_, err := s.stitches.GetByAbbreviation(ctx, st.Abbreviation, nil)
		if err == nil {
			continue // already exists
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("check stitch %s: %w", st.Abbreviation, err)
		}
		if err := s.stitches.Create(ctx, &st); err != nil {
			return fmt.Errorf("seed stitch %s: %w", st.Abbreviation, err)
		}
	}
	return nil
}

var predefinedStitches = []domain.Stitch{
	// Basic Stitches
	{Abbreviation: "ch", Name: "Chain", Category: "basic"},
	{Abbreviation: "sl st", Name: "Slip Stitch", Category: "basic"},
	{Abbreviation: "sc", Name: "Single Crochet", Category: "basic"},
	{Abbreviation: "hdc", Name: "Half Double Crochet", Category: "basic"},
	{Abbreviation: "dc", Name: "Double Crochet", Category: "basic"},
	{Abbreviation: "tr", Name: "Treble Crochet", Category: "basic"},
	{Abbreviation: "dtr", Name: "Double Treble Crochet", Category: "basic"},
	// Increase / Decrease
	{Abbreviation: "inc", Name: "Increase (2 stitches in one)", Category: "increase"},
	{Abbreviation: "dec", Name: "Decrease (2 stitches together)", Category: "decrease"},
	{Abbreviation: "sc2tog", Name: "Single Crochet 2 Together", Category: "decrease"},
	{Abbreviation: "hdc2tog", Name: "Half Double Crochet 2 Together", Category: "decrease"},
	{Abbreviation: "dc2tog", Name: "Double Crochet 2 Together", Category: "decrease"},
	{Abbreviation: "dc3tog", Name: "Double Crochet 3 Together", Category: "decrease"},
	{Abbreviation: "tr2tog", Name: "Treble Crochet 2 Together", Category: "decrease"},
	// Post Stitches
	{Abbreviation: "FPsc", Name: "Front Post Single Crochet", Category: "post"},
	{Abbreviation: "BPsc", Name: "Back Post Single Crochet", Category: "post"},
	{Abbreviation: "FPdc", Name: "Front Post Double Crochet", Category: "post"},
	{Abbreviation: "BPdc", Name: "Back Post Double Crochet", Category: "post"},
	{Abbreviation: "FPtr", Name: "Front Post Treble Crochet", Category: "post"},
	{Abbreviation: "BPtr", Name: "Back Post Treble Crochet", Category: "post"},
	// Loop Variations
	{Abbreviation: "BLO", Name: "Back Loop Only", Category: "advanced"},
	{Abbreviation: "FLO", Name: "Front Loop Only", Category: "advanced"},
	// Specialty Stitches
	{Abbreviation: "pc", Name: "Popcorn Stitch", Category: "specialty"},
	{Abbreviation: "puff", Name: "Puff Stitch", Category: "specialty"},
	{Abbreviation: "cl", Name: "Cluster", Category: "specialty"},
	{Abbreviation: "sh", Name: "Shell", Category: "specialty"},
	{Abbreviation: "bob", Name: "Bobble", Category: "specialty"},
	{Abbreviation: "crab st", Name: "Crab Stitch (Reverse SC)", Category: "specialty"},
	{Abbreviation: "lp st", Name: "Loop Stitch", Category: "specialty"},
	{Abbreviation: "v-st", Name: "V-Stitch", Category: "specialty"},
	// Action
	{Abbreviation: "sk", Name: "Skip", Category: "action"},
	{Abbreviation: "yo", Name: "Yarn Over", Category: "action"},
	{Abbreviation: "tch", Name: "Turning Chain", Category: "action"},
	{Abbreviation: "MR", Name: "Magic Ring", Category: "action"},
}
