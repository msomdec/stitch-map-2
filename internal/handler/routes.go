package handler

import (
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/service"
)

// RegisterRoutes sets up all HTTP routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, auth *service.AuthService, stitches *service.StitchService, patterns *service.PatternService, sessions *service.WorkSessionService) {
	authHandler := NewAuthHandler(auth)
	stitchHandler := NewStitchHandler(stitches)
	patternHandler := NewPatternHandler(patterns, stitches)
	sessionHandler := NewWorkSessionHandler(sessions, patterns, stitches)

	// Public routes.
	mux.HandleFunc("GET /healthz", HandleHealthz)
	mux.Handle("GET /{$}", OptionalAuth(auth, http.HandlerFunc(HandleHome)))

	// Auth routes (unauthenticated).
	mux.HandleFunc("GET /login", authHandler.ShowLogin)
	mux.HandleFunc("POST /login", authHandler.HandleLogin)
	mux.HandleFunc("GET /register", authHandler.ShowRegister)
	mux.HandleFunc("POST /register", authHandler.HandleRegister)
	mux.HandleFunc("POST /logout", authHandler.HandleLogout)

	// Protected routes.
	mux.Handle("GET /dashboard", RequireAuth(auth, http.HandlerFunc(HandleDashboard)))

	// Stitch library routes (authenticated).
	mux.Handle("GET /stitches", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleLibrary)))
	mux.Handle("POST /stitches", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleCreateCustom)))
	mux.Handle("POST /stitches/{id}/edit", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleUpdateCustom)))
	mux.Handle("POST /stitches/{id}/delete", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleDeleteCustom)))

	// Pattern routes (authenticated).
	mux.Handle("GET /patterns", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleList)))
	mux.Handle("GET /patterns/new", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleNew)))
	mux.Handle("POST /patterns", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleCreate)))
	mux.Handle("GET /patterns/{id}", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleView)))
	mux.Handle("GET /patterns/{id}/edit", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleEdit)))
	mux.Handle("POST /patterns/{id}/edit", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleUpdate)))
	mux.Handle("POST /patterns/{id}/delete", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleDelete)))
	mux.Handle("POST /patterns/{id}/duplicate", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleDuplicate)))

	// Work session routes (authenticated).
	mux.Handle("POST /patterns/{id}/start-session", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleStart)))
	mux.Handle("GET /sessions/{id}", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleView)))
	mux.Handle("POST /sessions/{id}/next", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleForward)))
	mux.Handle("POST /sessions/{id}/prev", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleBackward)))
	mux.Handle("POST /sessions/{id}/pause", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandlePause)))
	mux.Handle("POST /sessions/{id}/resume", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleResume)))
	mux.Handle("POST /sessions/{id}/abandon", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleAbandon)))
}
