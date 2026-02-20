package handler

import (
	"log/slog"
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
)

// DashboardHandler handles the dashboard page.
type DashboardHandler struct {
	sessions *service.WorkSessionService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(sessions *service.WorkSessionService) *DashboardHandler {
	return &DashboardHandler{sessions: sessions}
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

	view.DashboardPage(user.DisplayName, activeSessions).Render(r.Context(), w)
}
