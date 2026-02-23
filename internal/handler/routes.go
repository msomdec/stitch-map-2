package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/msomdec/stitch-map-2/internal/service"
)

// RegisterRoutes sets up all HTTP routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, auth *service.AuthService, stitches *service.StitchService, patterns *service.PatternService, sessions *service.WorkSessionService, images *service.ImageService) {
	authHandler := NewAuthHandler(auth)
	stitchHandler := NewStitchHandler(stitches)
	patternHandler := NewPatternHandler(patterns, stitches, images)
	sessionHandler := NewWorkSessionHandler(sessions, patterns, stitches)
	dashboardHandler := NewDashboardHandler(sessions, patterns)
	imageHandler := NewImageHandler(images, patterns)

	// Rate limiter for auth endpoints: 10 req/s capacity, refills at 1/s.
	authLimiter := service.NewTokenBucket(1, 10)

	// Public routes.
	mux.HandleFunc("GET /healthz", HandleHealthz)

	// Auth routes.
	mux.Handle("GET /api/auth/me", OptionalAuth(auth, http.HandlerFunc(authHandler.HandleMe)))
	mux.Handle("POST /api/auth/login", RateLimit(authLimiter, http.HandlerFunc(authHandler.HandleLogin)))
	mux.Handle("POST /api/auth/register", RateLimit(authLimiter, http.HandlerFunc(authHandler.HandleRegister)))
	mux.HandleFunc("POST /api/auth/logout", authHandler.HandleLogout)

	// Dashboard routes (authenticated).
	mux.Handle("GET /api/dashboard", RequireAuth(auth, http.HandlerFunc(dashboardHandler.HandleDashboard)))
	mux.Handle("GET /api/dashboard/completed", RequireAuth(auth, http.HandlerFunc(dashboardHandler.HandleLoadMoreCompleted)))

	// Stitch library routes (authenticated).
	mux.Handle("GET /api/stitches", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleLibrary)))
	mux.Handle("GET /api/stitches/all", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleListAll)))
	mux.Handle("POST /api/stitches", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleCreateCustom)))
	mux.Handle("PUT /api/stitches/{id}", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleUpdateCustom)))
	mux.Handle("DELETE /api/stitches/{id}", RequireAuth(auth, http.HandlerFunc(stitchHandler.HandleDeleteCustom)))

	// Pattern routes (authenticated).
	mux.Handle("GET /api/patterns", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleList)))
	mux.Handle("POST /api/patterns", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleCreate)))
	mux.Handle("POST /api/patterns/preview", RequireAuth(auth, http.HandlerFunc(patternHandler.HandlePreview)))
	mux.Handle("GET /api/patterns/{id}", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleView)))
	mux.Handle("PUT /api/patterns/{id}", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleUpdate)))
	mux.Handle("DELETE /api/patterns/{id}", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleDelete)))
	mux.Handle("POST /api/patterns/{id}/duplicate", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleDuplicate)))

	// Image routes (authenticated).
	mux.Handle("POST /api/patterns/{id}/groups/{groupIndex}/images", RequireAuth(auth, http.HandlerFunc(imageHandler.HandleUpload)))
	mux.Handle("GET /api/images/{id}", RequireAuth(auth, http.HandlerFunc(imageHandler.HandleServe)))
	mux.Handle("DELETE /api/images/{id}", RequireAuth(auth, http.HandlerFunc(imageHandler.HandleDelete)))

	// Work session routes (authenticated).
	mux.Handle("POST /api/patterns/{id}/sessions", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleStart)))
	mux.Handle("GET /api/sessions/{id}", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleView)))
	mux.Handle("POST /api/sessions/{id}/next", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleForward)))
	mux.Handle("POST /api/sessions/{id}/prev", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleBackward)))
	mux.Handle("POST /api/sessions/{id}/pause", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandlePause)))
	mux.Handle("POST /api/sessions/{id}/resume", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleResume)))
	mux.Handle("DELETE /api/sessions/{id}", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleAbandon)))

	// SPA fallback: serve index.html for non-API routes.
	mux.HandleFunc("/", HandleSPA)
}

// HandleSPA serves the React SPA. For requests to static files (JS, CSS, images),
// it serves the file directly. For all other non-API routes, it serves index.html
// so that React Router can handle client-side routing.
func HandleSPA(w http.ResponseWriter, r *http.Request) {
	// Determine the static files directory.
	// In production, the React build output is in ./frontend/dist.
	staticDir := filepath.Join("frontend", "dist")

	// Check if the requested file exists on disk.
	path := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		// Serve the static file directly.
		http.ServeFile(w, r, path)
		return
	}

	// For API routes that don't match, return 404 JSON.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}

	// For all other paths, serve index.html so React Router can handle routing.
	indexPath := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	http.ServeFile(w, r, indexPath)
}
