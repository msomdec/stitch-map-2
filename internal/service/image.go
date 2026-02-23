package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

const (
	maxImageSize     = 10 * 1024 * 1024 // 10MB
	maxImagesPerPart = 5
)

// ImageService orchestrates image uploads, retrieval, and deletion.
type ImageService struct {
	images   domain.PatternImageRepository
	files    domain.FileStore
	patterns domain.PatternRepository
}

// NewImageService creates a new ImageService.
func NewImageService(images domain.PatternImageRepository, files domain.FileStore, patterns domain.PatternRepository) *ImageService {
	return &ImageService{images: images, files: files, patterns: patterns}
}

// Upload validates and stores an image for an instruction group.
func (s *ImageService) Upload(ctx context.Context, userID, groupID int64, filename, contentType string, data []byte) (*domain.PatternImage, error) {
	// Validate content type.
	if contentType != "image/jpeg" && contentType != "image/png" {
		return nil, fmt.Errorf("%w: only JPEG and PNG images are accepted", domain.ErrInvalidInput)
	}

	// Validate file size.
	if len(data) > maxImageSize {
		return nil, fmt.Errorf("%w: image exceeds 10MB limit", domain.ErrInvalidInput)
	}

	// Check image count limit.
	count, err := s.images.CountByGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("count images: %w", err)
	}
	if count >= maxImagesPerPart {
		return nil, fmt.Errorf("%w: maximum %d images per part", domain.ErrInvalidInput, maxImagesPerPart)
	}

	// Generate a unique storage key.
	key, err := generateStorageKey()
	if err != nil {
		return nil, fmt.Errorf("generate storage key: %w", err)
	}

	// Save the file bytes.
	if err := s.files.Save(ctx, key, data); err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	// Create image metadata.
	image := &domain.PatternImage{
		InstructionGroupID: groupID,
		Filename:           filename,
		ContentType:        contentType,
		Size:               int64(len(data)),
		StorageKey:         key,
		SortOrder:          count, // Append at end
	}

	if err := s.images.Create(ctx, image); err != nil {
		// Best-effort cleanup of the stored file.
		s.files.Delete(ctx, key)
		return nil, fmt.Errorf("create image record: %w", err)
	}

	return image, nil
}

// GetFile returns the image bytes and content type after ownership check.
func (s *ImageService) GetFile(ctx context.Context, userID, imageID int64) ([]byte, string, error) {
	ownerID, err := s.images.GetOwnerUserID(ctx, imageID)
	if err != nil {
		return nil, "", fmt.Errorf("get image owner: %w", err)
	}
	if ownerID != userID {
		return nil, "", domain.ErrUnauthorized
	}

	image, err := s.images.GetByID(ctx, imageID)
	if err != nil {
		return nil, "", fmt.Errorf("get image: %w", err)
	}

	data, err := s.files.Get(ctx, image.StorageKey)
	if err != nil {
		return nil, "", fmt.Errorf("get file: %w", err)
	}

	return data, image.ContentType, nil
}

// Delete removes an image and its stored bytes after ownership check.
func (s *ImageService) Delete(ctx context.Context, userID, imageID int64) error {
	ownerID, err := s.images.GetOwnerUserID(ctx, imageID)
	if err != nil {
		return fmt.Errorf("get image owner: %w", err)
	}
	if ownerID != userID {
		return domain.ErrUnauthorized
	}

	image, err := s.images.GetByID(ctx, imageID)
	if err != nil {
		return fmt.Errorf("get image: %w", err)
	}

	// Delete stored bytes first, then metadata.
	if err := s.files.Delete(ctx, image.StorageKey); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	if err := s.images.Delete(ctx, imageID); err != nil {
		return fmt.Errorf("delete image record: %w", err)
	}

	return nil
}

// ListByGroup returns all images for an instruction group.
func (s *ImageService) ListByGroup(ctx context.Context, groupID int64) ([]domain.PatternImage, error) {
	return s.images.ListByGroup(ctx, groupID)
}

// ListByPattern returns all images for a pattern, keyed by instruction group ID.
func (s *ImageService) ListByPattern(ctx context.Context, pattern *domain.Pattern) (map[int64][]domain.PatternImage, error) {
	result := make(map[int64][]domain.PatternImage)
	for _, g := range pattern.InstructionGroups {
		images, err := s.images.ListByGroup(ctx, g.ID)
		if err != nil {
			return nil, fmt.Errorf("list images for group %d: %w", g.ID, err)
		}
		if len(images) > 0 {
			result[g.ID] = images
		}
	}
	return result, nil
}

func generateStorageKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pattern-images/" + hex.EncodeToString(b), nil
}
