package handler

import (
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/view"
)

// HandleHome renders the home page.
func HandleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	view.HomePage().Render(r.Context(), w)
}
