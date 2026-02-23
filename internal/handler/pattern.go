package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
)

// PatternHandler handles pattern-related HTTP requests.
type PatternHandler struct {
	patterns *service.PatternService
	stitches *service.StitchService
	images   *service.ImageService
}

// NewPatternHandler creates a new PatternHandler.
func NewPatternHandler(patterns *service.PatternService, stitches *service.StitchService, images *service.ImageService) *PatternHandler {
	return &PatternHandler{patterns: patterns, stitches: stitches, images: images}
}

// HandleList returns all patterns for the authenticated user.
// GET /api/patterns
// Response: {"patterns": [...]}
func (h *PatternHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	patterns, err := h.patterns.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list patterns", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"patterns": toPatternDTOs(patterns),
	})
}

// HandleView returns a single pattern with stitch count, rendered text, and group images.
// GET /api/patterns/{id}
// Response: {"pattern": {...}, "stitchCount": N, "patternText": "...", "groupImages": {...}}
func (h *PatternHandler) HandleView(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid pattern ID.")
		return
	}

	pattern, err := h.patterns.GetByID(r.Context(), id)
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

	allStitches, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stitches", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	groupImages, err := h.images.ListByPattern(r.Context(), pattern)
	if err != nil {
		slog.Error("list pattern images", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	// Convert groupImages map[int64][]domain.PatternImage to map[string][]PatternImageDTO
	// (JSON keys must be strings).
	groupImageDTOs := make(map[string][]PatternImageDTO)
	for groupID, images := range groupImages {
		groupImageDTOs[strconv.FormatInt(groupID, 10)] = toPatternImageDTOs(images)
	}

	stitchCount := service.StitchCount(pattern)
	patternText := service.RenderPatternText(pattern, allStitches)

	writeJSON(w, http.StatusOK, map[string]any{
		"pattern":     toPatternDTO(pattern),
		"stitchCount": stitchCount,
		"patternText": patternText,
		"groupImages": groupImageDTOs,
	})
}

// patternRequest is the JSON body for creating or updating a pattern.
type patternRequest struct {
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	PatternType       string                    `json:"patternType"`
	HookSize          string                    `json:"hookSize"`
	YarnWeight        string                    `json:"yarnWeight"`
	Difficulty        string                    `json:"difficulty"`
	InstructionGroups []instructionGroupRequest `json:"instructionGroups"`
}

type instructionGroupRequest struct {
	Label         string              `json:"label"`
	RepeatCount   int                 `json:"repeatCount"`
	ExpectedCount *int                `json:"expectedCount"`
	Notes         string              `json:"notes"`
	StitchEntries []stitchEntryRequest `json:"stitchEntries"`
}

type stitchEntryRequest struct {
	StitchID    int64  `json:"stitchId"`
	Count       int    `json:"count"`
	IntoStitch  string `json:"intoStitch"`
	RepeatCount int    `json:"repeatCount"`
}

func patternFromRequest(req patternRequest, userID int64) *domain.Pattern {
	patternType := domain.PatternType(req.PatternType)
	if patternType == "" {
		patternType = domain.PatternTypeRound
	}

	pattern := &domain.Pattern{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		PatternType: patternType,
		HookSize:    req.HookSize,
		YarnWeight:  req.YarnWeight,
		Difficulty:  req.Difficulty,
	}

	for i, g := range req.InstructionGroups {
		repeatCount := g.RepeatCount
		if repeatCount < 1 {
			repeatCount = 1
		}

		group := domain.InstructionGroup{
			SortOrder:     i,
			Label:         g.Label,
			RepeatCount:   repeatCount,
			ExpectedCount: g.ExpectedCount,
			Notes:         g.Notes,
		}

		for j, e := range g.StitchEntries {
			count := e.Count
			if count < 1 {
				count = 1
			}
			entryRepeat := e.RepeatCount
			if entryRepeat < 1 {
				entryRepeat = 1
			}

			entry := domain.StitchEntry{
				SortOrder:   j,
				StitchID:    e.StitchID,
				Count:       count,
				IntoStitch:  e.IntoStitch,
				RepeatCount: entryRepeat,
			}
			group.StitchEntries = append(group.StitchEntries, entry)
		}

		pattern.InstructionGroups = append(pattern.InstructionGroups, group)
	}

	return pattern
}

// HandleCreate creates a new pattern from a JSON body.
// POST /api/patterns
// Request: patternRequest JSON
// Response: {"pattern": {...}}
func (h *PatternHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var req patternRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	pattern := patternFromRequest(req, user.ID)

	if err := h.patterns.Create(r.Context(), pattern); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("create pattern", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"pattern": toPatternDTO(pattern),
	})
}

// HandleUpdate updates an existing pattern from a JSON body.
// PUT /api/patterns/{id}
// Request: patternRequest JSON
// Response: {"pattern": {...}}
func (h *PatternHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid pattern ID.")
		return
	}

	var req patternRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	pattern := patternFromRequest(req, user.ID)
	pattern.ID = id

	if err := h.patterns.Update(r.Context(), user.ID, pattern); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pattern not found.")
			return
		}
		slog.Error("update pattern", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	// Re-fetch to get the updated timestamps and IDs.
	updated, err := h.patterns.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get pattern after update", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pattern": toPatternDTO(updated),
	})
}

// HandleDelete deletes a pattern.
// DELETE /api/patterns/{id}
// Response: 204 No Content
func (h *PatternHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid pattern ID.")
		return
	}

	if err := h.patterns.Delete(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pattern not found.")
			return
		}
		slog.Error("delete pattern", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDuplicate duplicates an existing pattern.
// POST /api/patterns/{id}/duplicate
// Response: {"pattern": {...}}
func (h *PatternHandler) HandleDuplicate(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid pattern ID.")
		return
	}

	dup, err := h.patterns.Duplicate(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pattern not found.")
			return
		}
		slog.Error("duplicate pattern", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"pattern": toPatternDTO(dup),
	})
}

// HandlePreview renders a pattern as text without saving it.
// POST /api/patterns/preview
// Request: patternRequest JSON
// Response: {"text": "...", "stitchCount": N}
func (h *PatternHandler) HandlePreview(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var req patternRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	pattern := patternFromRequest(req, user.ID)

	allStitches, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stitches for preview", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	text := service.RenderPatternText(pattern, allStitches)
	stitchCount := service.StitchCount(pattern)

	writeJSON(w, http.StatusOK, map[string]any{
		"text":        text,
		"stitchCount": stitchCount,
	})
}
