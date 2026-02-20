package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
)

// StitchHandler handles stitch library HTTP requests.
type StitchHandler struct {
	stitches *service.StitchService
}

// NewStitchHandler creates a new StitchHandler.
func NewStitchHandler(stitches *service.StitchService) *StitchHandler {
	return &StitchHandler{stitches: stitches}
}

// HandleLibrary renders the stitch library page showing predefined and user custom stitches.
func (h *StitchHandler) HandleLibrary(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	predefined, err := h.stitches.ListPredefined(r.Context())
	if err != nil {
		slog.Error("list predefined stitches", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	custom, err := h.stitches.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list user stitches", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	predefined = filterStitches(predefined, category, search)
	custom = filterStitches(custom, category, search)

	view.StitchLibraryPage(user.DisplayName, predefined, custom, category, search, "").Render(r.Context(), w)
}

// HandleCreateCustom processes custom stitch creation.
func (h *StitchHandler) HandleCreateCustom(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	abbreviation := r.FormValue("abbreviation")
	name := r.FormValue("name")
	description := r.FormValue("description")
	category := r.FormValue("category")

	_, err := h.stitches.CreateCustom(r.Context(), user.ID, abbreviation, name, description, category)
	if err != nil {
		errMsg := handleStitchError(err)
		h.renderLibraryWithError(w, r, user, errMsg)
		return
	}

	http.Redirect(w, r, "/stitches", http.StatusSeeOther)
}

// HandleUpdateCustom processes custom stitch updates.
func (h *StitchHandler) HandleUpdateCustom(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	abbreviation := r.FormValue("abbreviation")
	name := r.FormValue("name")
	description := r.FormValue("description")
	category := r.FormValue("category")

	_, err = h.stitches.UpdateCustom(r.Context(), user.ID, id, abbreviation, name, description, category)
	if err != nil {
		errMsg := handleStitchError(err)
		h.renderLibraryWithError(w, r, user, errMsg)
		return
	}

	http.Redirect(w, r, "/stitches", http.StatusSeeOther)
}

// HandleDeleteCustom processes custom stitch deletion.
func (h *StitchHandler) HandleDeleteCustom(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.stitches.DeleteCustom(r.Context(), user.ID, id); err != nil {
		errMsg := handleStitchError(err)
		h.renderLibraryWithError(w, r, user, errMsg)
		return
	}

	http.Redirect(w, r, "/stitches", http.StatusSeeOther)
}

func (h *StitchHandler) renderLibraryWithError(w http.ResponseWriter, r *http.Request, user *domain.User, errMsg string) {
	predefined, _ := h.stitches.ListPredefined(r.Context())
	custom, _ := h.stitches.ListByUser(r.Context(), user.ID)

	w.WriteHeader(http.StatusUnprocessableEntity)
	view.StitchLibraryPage(user.DisplayName, predefined, custom, "", "", errMsg).Render(r.Context(), w)
}

func handleStitchError(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return err.Error()
	case errors.Is(err, domain.ErrReservedAbbreviation):
		return err.Error()
	case errors.Is(err, domain.ErrDuplicateAbbreviation):
		return "A stitch with that abbreviation already exists."
	case errors.Is(err, domain.ErrUnauthorized):
		return "You can only modify your own custom stitches."
	case errors.Is(err, domain.ErrNotFound):
		return "Stitch not found."
	default:
		slog.Error("stitch operation", "error", err)
		return "An unexpected error occurred. Please try again."
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
