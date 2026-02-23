package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
)

// ImageHandler handles image upload, retrieval, and deletion.
type ImageHandler struct {
	images   *service.ImageService
	patterns *service.PatternService
}

// NewImageHandler creates a new ImageHandler.
func NewImageHandler(images *service.ImageService, patterns *service.PatternService) *ImageHandler {
	return &ImageHandler{images: images, patterns: patterns}
}

// HandleUpload processes a multipart image upload for a pattern group.
// POST /api/patterns/{id}/groups/{groupIndex}/images
// Response: {"image": {...}}
func (h *ImageHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	patternID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid pattern ID.")
		return
	}

	groupIndex, err := strconv.Atoi(r.PathValue("groupIndex"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid group index.")
		return
	}

	// Verify pattern ownership and resolve the instruction group.
	pattern, err := h.patterns.GetByID(r.Context(), patternID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pattern not found.")
			return
		}
		slog.Error("get pattern", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}
	if pattern.UserID != user.ID {
		writeError(w, http.StatusNotFound, "Pattern not found.")
		return
	}
	if groupIndex < 0 || groupIndex >= len(pattern.InstructionGroups) {
		writeError(w, http.StatusBadRequest, "Invalid group index.")
		return
	}

	group := &pattern.InstructionGroups[groupIndex]

	// Parse multipart form (10MB limit).
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "File too large.")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No image file provided.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("read upload", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	// Detect content type from file bytes (more reliable than multipart header).
	contentType := http.DetectContentType(data)

	image, err := h.images.Upload(r.Context(), user.ID, group.ID, header.Filename, contentType, data)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		slog.Error("upload image", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"image": toPatternImageDTO(*image),
	})
}

// HandleServe serves image bytes with correct Content-Type.
// GET /api/images/{id}
func (h *ImageHandler) HandleServe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	imageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid image ID.")
		return
	}

	data, contentType, err := h.images.GetFile(r.Context(), user.ID, imageID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusNotFound, "Image not found.")
			return
		}
		slog.Error("serve image", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// HandleDelete deletes an image.
// DELETE /api/images/{id}
// Response: 204 No Content
func (h *ImageHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	imageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid image ID.")
		return
	}

	if err := h.images.Delete(r.Context(), user.ID, imageID); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusNotFound, "Image not found.")
			return
		}
		slog.Error("delete image", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
