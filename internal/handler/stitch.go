package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
)

// StitchHandler handles stitch library HTTP requests.
type StitchHandler struct {
	stitches *service.StitchService
}

// NewStitchHandler creates a new StitchHandler.
func NewStitchHandler(stitches *service.StitchService) *StitchHandler {
	return &StitchHandler{stitches: stitches}
}

// HandleLibrary returns predefined and user custom stitches, optionally filtered.
// GET /api/stitches?category=X&search=Y
// Response: {"predefined": [...], "custom": [...]}
func (h *StitchHandler) HandleLibrary(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	predefined, err := h.stitches.ListPredefined(r.Context())
	if err != nil {
		slog.Error("list predefined stitches", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	custom, err := h.stitches.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list user stitches", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	predefined = filterStitches(predefined, category, search)
	custom = filterStitches(custom, category, search)

	writeJSON(w, http.StatusOK, map[string]any{
		"predefined": toStitchDTOs(predefined),
		"custom":     toStitchDTOs(custom),
	})
}

// HandleListAll returns all stitches (predefined + user's custom) as a single list.
// GET /api/stitches/all
// Response: {"stitches": [...]}
func (h *StitchHandler) HandleListAll(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	all, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list all stitches", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stitches": toStitchDTOs(all),
	})
}

// HandleCreateCustom creates a new custom stitch from a JSON body.
// POST /api/stitches
// Request: {"abbreviation":"...","name":"...","description":"...","category":"..."}
// Response: {"stitch": {...}}
func (h *StitchHandler) HandleCreateCustom(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	var req struct {
		Abbreviation string `json:"abbreviation"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		Category     string `json:"category"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	stitch, err := h.stitches.CreateCustom(r.Context(), user.ID, req.Abbreviation, req.Name, req.Description, req.Category)
	if err != nil {
		status, msg := stitchErrorResponse(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"stitch": toStitchDTO(*stitch),
	})
}

// HandleUpdateCustom updates an existing custom stitch from a JSON body.
// PUT /api/stitches/{id}
// Request: {"abbreviation":"...","name":"...","description":"...","category":"..."}
// Response: {"stitch": {...}}
func (h *StitchHandler) HandleUpdateCustom(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid stitch ID.")
		return
	}

	var req struct {
		Abbreviation string `json:"abbreviation"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		Category     string `json:"category"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	stitch, err := h.stitches.UpdateCustom(r.Context(), user.ID, id, req.Abbreviation, req.Name, req.Description, req.Category)
	if err != nil {
		status, msg := stitchErrorResponse(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stitch": toStitchDTO(*stitch),
	})
}

// HandleDeleteCustom deletes a custom stitch.
// DELETE /api/stitches/{id}
// Response: 204 No Content
func (h *StitchHandler) HandleDeleteCustom(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid stitch ID.")
		return
	}

	if err := h.stitches.DeleteCustom(r.Context(), user.ID, id); err != nil {
		status, msg := stitchErrorResponse(err)
		writeError(w, status, msg)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// stitchErrorResponse maps stitch service errors to HTTP status codes and messages.
func stitchErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, domain.ErrReservedAbbreviation):
		return http.StatusConflict, err.Error()
	case errors.Is(err, domain.ErrDuplicateAbbreviation):
		return http.StatusConflict, "A stitch with that abbreviation already exists."
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusForbidden, "You can only modify your own custom stitches."
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "Stitch not found."
	default:
		slog.Error("stitch operation", "error", err)
		return http.StatusInternalServerError, "An unexpected error occurred. Please try again."
	}
}

func filterStitches(stitches []domain.Stitch, category, search string) []domain.Stitch {
	if category == "" && search == "" {
		return stitches
	}

	var filtered []domain.Stitch
	for _, s := range stitches {
		if category != "" && s.Category != category {
			continue
		}
		if search != "" && !matchesSearch(s, search) {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered
}

func matchesSearch(s domain.Stitch, search string) bool {
	search = strings.ToLower(search)
	return strings.Contains(strings.ToLower(s.Abbreviation), search) ||
		strings.Contains(strings.ToLower(s.Name), search) ||
		strings.Contains(strings.ToLower(s.Description), search)
}
