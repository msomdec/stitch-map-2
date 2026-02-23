package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
	"github.com/starfederation/datastar-go/datastar"
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

// HandleUpload processes a multipart image upload for a pattern part.
// Called via JavaScript fetch, returns plain HTTP (not SSE).
// POST /patterns/{id}/parts/{groupIndex}/images
func (h *ImageHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	patternID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	groupIndex, err := strconv.Atoi(r.PathValue("groupIndex"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Verify pattern ownership and resolve the instruction group.
	pattern, err := h.patterns.GetByID(r.Context(), patternID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("get pattern", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if pattern.UserID != user.ID {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if groupIndex < 0 || groupIndex >= len(pattern.InstructionGroups) {
		http.Error(w, "Bad Request: invalid group index", http.StatusBadRequest)
		return
	}

	group := &pattern.InstructionGroups[groupIndex]

	// Parse multipart form (10MB limit).
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "No image file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("read upload", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Detect content type from file bytes (more reliable than multipart header).
	contentType := http.DetectContentType(data)

	_, err = h.images.Upload(r.Context(), user.ID, group.ID, header.Filename, contentType, data)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("upload image", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleServe serves image bytes with correct Content-Type.
// GET /images/{id}
func (h *ImageHandler) HandleServe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	imageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	data, contentType, err := h.images.GetFile(r.Context(), user.ID, imageID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("serve image", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// HandleDelete deletes an image and responds with SSE to update the UI.
// POST /images/{id}/delete
func (h *ImageHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	imageID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Read context params from query so we can re-render the image section.
	patternID, _ := strconv.ParseInt(r.URL.Query().Get("patternID"), 10, 64)
	groupIndex, _ := strconv.Atoi(r.URL.Query().Get("groupIndex"))
	groupID, _ := strconv.ParseInt(r.URL.Query().Get("groupID"), 10, 64)

	if err := h.images.Delete(r.Context(), user.ID, imageID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("delete image", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Re-render the image section via SSE.
	images, err := h.images.ListByGroup(r.Context(), groupID)
	if err != nil {
		slog.Error("list images after delete", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(
		view.ImageSection(patternID, groupIndex, groupID, images),
		datastar.WithSelectorID("images-"+strconv.Itoa(groupIndex)),
		datastar.WithModeInner(),
	)
}
