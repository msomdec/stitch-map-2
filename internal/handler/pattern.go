package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
)

// PatternHandler handles pattern-related HTTP requests.
type PatternHandler struct {
	patterns *service.PatternService
	stitches *service.StitchService
}

// NewPatternHandler creates a new PatternHandler.
func NewPatternHandler(patterns *service.PatternService, stitches *service.StitchService) *PatternHandler {
	return &PatternHandler{patterns: patterns, stitches: stitches}
}

// HandleList renders the pattern list page.
func (h *PatternHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	patterns, err := h.patterns.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list patterns", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	view.PatternListPage(user.DisplayName, patterns).Render(r.Context(), w)
}

// HandleNew renders the pattern creation form.
func (h *PatternHandler) HandleNew(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	allStitches, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stitches", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	view.PatternEditorPage(user.DisplayName, nil, allStitches, "").Render(r.Context(), w)
}

// HandleCreate processes pattern creation from the form.
func (h *PatternHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pattern, err := parsePatternForm(r, user.ID)
	if err != nil {
		h.renderEditorWithError(w, r, user, nil, err.Error())
		return
	}

	if err := h.patterns.Create(r.Context(), pattern); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			h.renderEditorWithError(w, r, user, pattern, err.Error())
			return
		}
		slog.Error("create pattern", "error", err)
		h.renderEditorWithError(w, r, user, pattern, "An unexpected error occurred.")
		return
	}

	http.Redirect(w, r, "/patterns", http.StatusSeeOther)
}

// HandleView renders a read-only pattern detail view.
func (h *PatternHandler) HandleView(w http.ResponseWriter, r *http.Request) {
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

	pattern, err := h.patterns.GetByID(r.Context(), id)
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

	allStitches, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stitches", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	view.PatternViewPage(user.DisplayName, pattern, allStitches).Render(r.Context(), w)
}

// HandleEdit renders the pattern editor for an existing pattern.
func (h *PatternHandler) HandleEdit(w http.ResponseWriter, r *http.Request) {
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

	pattern, err := h.patterns.GetByID(r.Context(), id)
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

	allStitches, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stitches", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	view.PatternEditorPage(user.DisplayName, pattern, allStitches, "").Render(r.Context(), w)
}

// HandleUpdate processes pattern update from the form.
func (h *PatternHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
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

	pattern, err := parsePatternForm(r, user.ID)
	if err != nil {
		h.renderEditorWithError(w, r, user, nil, err.Error())
		return
	}
	pattern.ID = id

	if err := h.patterns.Update(r.Context(), user.ID, pattern); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			h.renderEditorWithError(w, r, user, pattern, err.Error())
			return
		}
		if errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("update pattern", "error", err)
		h.renderEditorWithError(w, r, user, pattern, "An unexpected error occurred.")
		return
	}

	http.Redirect(w, r, "/patterns", http.StatusSeeOther)
}

// HandleDelete processes pattern deletion.
func (h *PatternHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.patterns.Delete(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("delete pattern", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/patterns", http.StatusSeeOther)
}

// HandleDuplicate duplicates an existing pattern.
func (h *PatternHandler) HandleDuplicate(w http.ResponseWriter, r *http.Request) {
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

	_, err = h.patterns.Duplicate(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("duplicate pattern", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/patterns", http.StatusSeeOther)
}

func (h *PatternHandler) renderEditorWithError(w http.ResponseWriter, r *http.Request, user *domain.User, pattern *domain.Pattern, errMsg string) {
	allStitches, _ := h.stitches.ListAll(r.Context(), user.ID)
	w.WriteHeader(http.StatusUnprocessableEntity)
	view.PatternEditorPage(user.DisplayName, pattern, allStitches, errMsg).Render(r.Context(), w)
}

// parsePatternForm reads pattern data from a form submission.
// The form uses indexed field names for nested groups and entries:
// group_label_0, group_repeat_0, group_expected_0
// entry_stitch_0_0, entry_count_0_0, entry_repeat_0_0, entry_into_0_0, entry_notes_0_0
func parsePatternForm(r *http.Request, userID int64) (*domain.Pattern, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	patternType := domain.PatternType(r.FormValue("pattern_type"))
	if patternType == "" {
		patternType = domain.PatternTypeRound
	}

	pattern := &domain.Pattern{
		UserID:      userID,
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		PatternType: patternType,
		HookSize:    r.FormValue("hook_size"),
		YarnWeight:  r.FormValue("yarn_weight"),
		Notes:       r.FormValue("notes"),
	}

	// Parse instruction groups.
	for gi := 0; ; gi++ {
		label := r.FormValue("group_label_" + strconv.Itoa(gi))
		if label == "" {
			break
		}

		repeatCount := intFormValue(r, "group_repeat_"+strconv.Itoa(gi), 1)
		var expectedCount *int
		if v := r.FormValue("group_expected_" + strconv.Itoa(gi)); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				expectedCount = &parsed
			}
		}

		group := domain.InstructionGroup{
			SortOrder:     gi,
			Label:         label,
			RepeatCount:   repeatCount,
			ExpectedCount: expectedCount,
		}

		// Parse stitch entries for this group.
		for ei := 0; ; ei++ {
			stitchIDStr := r.FormValue("entry_stitch_" + strconv.Itoa(gi) + "_" + strconv.Itoa(ei))
			if stitchIDStr == "" {
				break
			}

			stitchID, err := strconv.ParseInt(stitchIDStr, 10, 64)
			if err != nil {
				continue
			}

			entry := domain.StitchEntry{
				SortOrder:   ei,
				StitchID:    stitchID,
				Count:       intFormValue(r, "entry_count_"+strconv.Itoa(gi)+"_"+strconv.Itoa(ei), 1),
				IntoStitch:  r.FormValue("entry_into_" + strconv.Itoa(gi) + "_" + strconv.Itoa(ei)),
				RepeatCount: intFormValue(r, "entry_repeat_"+strconv.Itoa(gi)+"_"+strconv.Itoa(ei), 1),
				Notes:       r.FormValue("entry_notes_" + strconv.Itoa(gi) + "_" + strconv.Itoa(ei)),
			}

			group.StitchEntries = append(group.StitchEntries, entry)
		}

		pattern.InstructionGroups = append(pattern.InstructionGroups, group)
	}

	return pattern, nil
}

func intFormValue(r *http.Request, key string, defaultVal int) int {
	v := r.FormValue(key)
	if v == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(v)
	if err != nil || parsed < 1 {
		return defaultVal
	}
	return parsed
}
