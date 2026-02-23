package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
	datastar "github.com/starfederation/datastar-go/datastar"
)

const completedPageSize = 5

// DashboardHandler handles the dashboard page.
type DashboardHandler struct {
	sessions *service.WorkSessionService
	patterns *service.PatternService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(sessions *service.WorkSessionService, patterns *service.PatternService) *DashboardHandler {
	return &DashboardHandler{sessions: sessions, patterns: patterns}
}

// HandleDashboard renders the dashboard page with the user's active and completed sessions.
func (h *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	activeSessions, err := h.sessions.GetActiveByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("get active sessions for dashboard", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	completedSessions, err := h.sessions.GetCompletedByUser(r.Context(), user.ID, completedPageSize, 0)
	if err != nil {
		slog.Error("get completed sessions for dashboard", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	totalCompleted, err := h.sessions.CountCompletedByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("count completed sessions for dashboard", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	patternNames := h.buildPatternNames(r, activeSessions, completedSessions)

	view.DashboardPage(user.DisplayName, activeSessions, completedSessions, patternNames, totalCompleted).Render(r.Context(), w)
}

// HandleLoadMoreCompleted returns additional completed sessions via SSE.
func (h *DashboardHandler) HandleLoadMoreCompleted(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	totalCompleted, err := h.sessions.CountCompletedByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("count completed sessions", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	patternNames := h.buildPatternNames(r, nil, completedSessions)
	nextOffset := offset + completedPageSize

	sse := datastar.NewSSE(w, r)

	// Append new session cards to the list.
	sse.PatchElementTempl(
		view.CompletedSessionsFragment(completedSessions, patternNames),
		datastar.WithSelectorID("completed-sessions-list"),
		datastar.WithModeAppend(),
	)

	// Replace the load-more button (updates count or removes it).
	sse.PatchElementTempl(
		view.LoadMoreFragment(totalCompleted, nextOffset),
	)
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
