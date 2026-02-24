package handler

import (
	"log/slog"
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
)

// DashboardHandler handles the dashboard page.
type DashboardHandler struct {
	sessions *service.WorkSessionService
	patterns *service.PatternService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(sessions *service.WorkSessionService, patterns *service.PatternService) *DashboardHandler {
	return &DashboardHandler{sessions: sessions, patterns: patterns}
}

// HandleDashboard renders the dashboard page with the user's active sessions.
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

	patternNames := h.buildPatternNames(r, activeSessions)

	view.DashboardPage(user.DisplayName, activeSessions, patternNames).Render(r.Context(), w)
}

// buildPatternNames builds a map of pattern ID to pattern name for the given sessions.
func (h *DashboardHandler) buildPatternNames(r *http.Request, sessions []domain.WorkSession) map[int64]string {
	seen := make(map[int64]bool)
	names := make(map[int64]string)

	for _, s := range sessions {
		if !seen[s.PatternID] {
			seen[s.PatternID] = true
			if p, err := h.patterns.GetByID(r.Context(), s.PatternID); err == nil {
				names[s.PatternID] = p.Name
			}
		}
	}

	return names
}
