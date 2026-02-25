package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/handler"
)

func TestHandleHome(t *testing.T) {
	auth, stitches, patterns, sessions, images, shares, users := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images, shares, users)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	auth, stitches, patterns, sessions, images, shares, users := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images, shares, users)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
