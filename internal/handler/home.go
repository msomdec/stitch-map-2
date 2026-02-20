package handler

import (
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/view"
)

// HandleHome renders the home page.
func HandleHome(w http.ResponseWriter, r *http.Request) {
	displayName := ""
	if user := UserFromContext(r.Context()); user != nil {
		displayName = user.DisplayName
	}
	view.HomePage(displayName).Render(r.Context(), w)
}
