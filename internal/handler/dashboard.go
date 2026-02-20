package handler

import (
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/view"
)

// HandleDashboard renders the dashboard page for authenticated users.
func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	view.DashboardPage(user.DisplayName).Render(r.Context(), w)
}
