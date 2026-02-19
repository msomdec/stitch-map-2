package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/handler"
)

func TestHandleHome(t *testing.T) {
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

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

func TestHandleHomeNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.HandleHome(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
