package handler

import (
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/service"
)

// RegisterRoutes sets up all HTTP routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, auth *service.AuthService) {
	authHandler := NewAuthHandler(auth)

	// Public routes.
	mux.HandleFunc("GET /healthz", HandleHealthz)
	mux.Handle("GET /{$}", OptionalAuth(auth, http.HandlerFunc(HandleHome)))

	// Auth routes (unauthenticated).
	mux.HandleFunc("GET /login", authHandler.ShowLogin)
	mux.HandleFunc("POST /login", authHandler.HandleLogin)
	mux.HandleFunc("GET /register", authHandler.ShowRegister)
	mux.HandleFunc("POST /register", authHandler.HandleRegister)
	mux.HandleFunc("POST /logout", authHandler.HandleLogout)

	// Protected routes (placeholder for future phases).
	mux.Handle("GET /dashboard", RequireAuth(auth, http.HandlerFunc(HandleDashboard)))
}
