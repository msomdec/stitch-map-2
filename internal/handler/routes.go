package handler

import (
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
)

// RegisterRoutes sets up all HTTP routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, auth *service.AuthService, stitches *service.StitchService, patterns *service.PatternService, sessions *service.WorkSessionService, images *service.ImageService) {
	authHandler := NewAuthHandler(auth)
	stitchHandler := NewStitchHandler(stitches)
	patternHandler := NewPatternHandler(patterns, stitches, images)
	sessionHandler := NewWorkSessionHandler(sessions, patterns, images)
	dashboardHandler := NewDashboardHandler(sessions, patterns)
	imageHandler := NewImageHandler(images, patterns)

	// Rate limiter for auth endpoints: 10 req/s capacity, refills at 1/s.
	authLimiter := service.NewTokenBucket(1, 10)

	// Public routes.
	mux.HandleFunc("GET /healthz", HandleHealthz)
	mux.Handle("GET /{$}", OptionalAuth(auth, http.HandlerFunc(HandleHome)))

	// Auth routes (unauthenticated, rate-limited).
	mux.Handle("GET /login", RateLimit(authLimiter, http.HandlerFunc(authHandler.ShowLogin)))
	mux.Handle("POST /login", RateLimit(authLimiter, http.HandlerFunc(authHandler.HandleLogin)))
	mux.Handle("GET /register", RateLimit(authLimiter, http.HandlerFunc(authHandler.ShowRegister)))
	mux.Handle("POST /register", RateLimit(authLimiter, http.HandlerFunc(authHandler.HandleRegister)))
	mux.HandleFunc("POST /logout", authHandler.HandleLogout)

	// Protected routes.
	mux.Handle("GET /dashboard", RequireAuth(auth, http.HandlerFunc(dashboardHandler.HandleDashboard)))


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

	// Pattern editor SSE endpoints (dynamic add/remove parts and entries).
	mux.Handle("POST /patterns/editor/add-part", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleAddPart)))
	mux.Handle("POST /patterns/editor/remove-part/{gi}", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleRemovePart)))
	mux.Handle("POST /patterns/editor/add-entry/{gi}", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleAddEntry)))
	mux.Handle("POST /patterns/editor/remove-entry/{gi}/{ei}", RequireAuth(auth, http.HandlerFunc(patternHandler.HandleRemoveEntry)))

	// Image routes (authenticated).
	mux.Handle("POST /patterns/{id}/parts/{groupIndex}/images", RequireAuth(auth, http.HandlerFunc(imageHandler.HandleUpload)))
	mux.Handle("GET /images/{id}", RequireAuth(auth, http.HandlerFunc(imageHandler.HandleServe)))
	mux.Handle("POST /images/{id}/delete", RequireAuth(auth, http.HandlerFunc(imageHandler.HandleDelete)))

	// Work session routes (authenticated).
	mux.Handle("POST /patterns/{id}/start-session", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleStart)))
	mux.Handle("GET /sessions/{id}", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleView)))
	mux.Handle("POST /sessions/{id}/next", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleForward)))
	mux.Handle("POST /sessions/{id}/prev", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleBackward)))
	mux.Handle("POST /sessions/{id}/pause", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandlePause)))
	mux.Handle("POST /sessions/{id}/resume", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleResume)))
	mux.Handle("POST /sessions/{id}/abandon", RequireAuth(auth, http.HandlerFunc(sessionHandler.HandleAbandon)))

	// Catch-all 404 handler.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		view.ErrorPage(http.StatusNotFound, "Page Not Found", "The page you're looking for doesn't exist.").Render(r.Context(), w)
	})
}
