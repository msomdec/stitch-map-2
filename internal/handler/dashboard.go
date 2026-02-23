package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
)

const completedPageSize = 5

// DashboardHandler handles the dashboard API endpoints.
type DashboardHandler struct {
	sessions *service.WorkSessionService
	patterns *service.PatternService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(sessions *service.WorkSessionService, patterns *service.PatternService) *DashboardHandler {
	return &DashboardHandler{sessions: sessions, patterns: patterns}
}

// HandleDashboard returns the user's active sessions, recent completed sessions,
// pattern names, and total completed count.
// GET /api/dashboard
// Response: {"activeSessions": [...], "completedSessions": [...], "patternNames": {...}, "totalCompleted": N}
func (h *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	activeSessions, err := h.sessions.GetActiveByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("get active sessions for dashboard", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	completedSessions, err := h.sessions.GetCompletedByUser(r.Context(), user.ID, completedPageSize, 0)
	if err != nil {
		slog.Error("get completed sessions for dashboard", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	totalCompleted, err := h.sessions.CountCompletedByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("count completed sessions for dashboard", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	patternNames := h.buildPatternNames(r, activeSessions, completedSessions)

	// Convert patternNames map keys to strings for JSON compatibility.
	patternNameStrs := make(map[string]string, len(patternNames))
	for id, name := range patternNames {
		patternNameStrs[strconv.FormatInt(id, 10)] = name
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"activeSessions":    toWorkSessionDTOs(activeSessions),
		"completedSessions": toWorkSessionDTOs(completedSessions),
		"patternNames":      patternNameStrs,
		"totalCompleted":    totalCompleted,
	})
}

// HandleLoadMoreCompleted returns additional completed sessions with pagination.
// GET /api/dashboard/completed?offset=N
// Response: {"sessions": [...], "patternNames": {...}, "totalCompleted": N}
func (h *DashboardHandler) HandleLoadMoreCompleted(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			offset = parsed
		}
	}

	completedSessions, err := h.sessions.GetCompletedByUser(r.Context(), user.ID, completedPageSize, offset)
	if err != nil {
		slog.Error("load more completed sessions", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	totalCompleted, err := h.sessions.CountCompletedByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("count completed sessions", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	patternNames := h.buildPatternNames(r, nil, completedSessions)

	patternNameStrs := make(map[string]string, len(patternNames))
	for id, name := range patternNames {
		patternNameStrs[strconv.FormatInt(id, 10)] = name
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions":       toWorkSessionDTOs(completedSessions),
		"patternNames":   patternNameStrs,
		"totalCompleted": totalCompleted,
	})
}

// buildPatternNames builds a map of pattern ID to pattern name for the given sessions.
func (h *DashboardHandler) buildPatternNames(r *http.Request, activeSessions, completedSessions []domain.WorkSession) map[int64]string {
	seen := make(map[int64]bool)
	names := make(map[int64]string)

	for _, s := range activeSessions {
		if !seen[s.PatternID] {
			seen[s.PatternID] = true
			if p, err := h.patterns.GetByID(r.Context(), s.PatternID); err == nil {
				names[s.PatternID] = p.Name
			}
		}
	}
	for _, s := range completedSessions {
		if !seen[s.PatternID] {
			seen[s.PatternID] = true
			if p, err := h.patterns.GetByID(r.Context(), s.PatternID); err == nil {
				names[s.PatternID] = p.Name
			}
		}
	}

	return names
}
