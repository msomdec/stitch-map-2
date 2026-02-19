package handler

import (
	"net/http"
)

// RegisterRoutes sets up all HTTP routes on the given mux.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", HandleHealthz)
	mux.HandleFunc("GET /", HandleHome)
}
