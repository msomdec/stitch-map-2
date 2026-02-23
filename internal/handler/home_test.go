package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/handler"
)

func TestSPAFallback_NoStaticFiles(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// When no frontend/dist exists, non-API routes should return 404.
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUnknownAPIPathReturns404(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/nonexistent")
	if err != nil {
		t.Fatalf("GET /api/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
