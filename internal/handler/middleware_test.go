package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/handler"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
	"github.com/msomdec/stitch-map-2/internal/service"
)

const testJWTSecret = "test-secret-for-handler-tests"

func newTestServices(t *testing.T) (*service.AuthService, *service.StitchService, *service.PatternService, *service.WorkSessionService, *service.ImageService, *service.ShareService, domain.UserRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New DB: %v", err)
	}
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return service.NewAuthService(db.Users(), testJWTSecret, 4),
		service.NewStitchService(db.Stitches()),
		service.NewPatternService(db.Patterns(), db.Stitches()),
		service.NewWorkSessionService(db.Sessions(), db.Patterns()),
		service.NewImageService(db.PatternImages(), db.FileStore(), db.Patterns()),
		service.NewShareService(db.Shares(), db.Patterns(), db.Users()),
		db.Users()
}

func TestRequireAuth_ValidJWT(t *testing.T) {
	auth, _, _, _, _, _, _ := newTestServices(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "valid@example.com", "Valid User", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	token, err := auth.Login(ctx, "valid@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	var gotUser string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := handler.UserFromContext(r.Context())
		if user != nil {
			gotUser = user.DisplayName
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()

	handler.RequireAuth(auth, inner).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser != "Valid User" {
		t.Fatalf("expected user 'Valid User', got %q", gotUser)
	}
}

func TestRequireAuth_MissingCookie(t *testing.T) {
	auth, _, _, _, _, _, _ := newTestServices(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()

	handler.RequireAuth(auth, inner).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_ExpiredOrInvalidToken(t *testing.T) {
	auth, _, _, _, _, _, _ := newTestServices(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid.jwt.token"})
	w := httptest.NewRecorder()

	handler.RequireAuth(auth, inner).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_TamperedToken(t *testing.T) {
	auth, _, _, _, _, _, _ := newTestServices(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "tamper@example.com", "Tamper", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	token, err := auth.Login(ctx, "tamper@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Tamper in the middle of the signature (not the last char, which may have
	// base64url padding bits that make single-char changes a no-op).
	mid := len(token) / 2
	replacement := byte('A')
	if token[mid] == 'A' {
		replacement = 'B'
	}
	tampered := token[:mid] + string(replacement) + token[mid+1:]

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: tampered})
	w := httptest.NewRecorder()

	handler.RequireAuth(auth, inner).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestOptionalAuth_WithToken(t *testing.T) {
	auth, _, _, _, _, _, _ := newTestServices(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "opt@example.com", "Optional", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	token, err := auth.Login(ctx, "opt@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	var gotUser string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := handler.UserFromContext(r.Context())
		if user != nil {
			gotUser = user.DisplayName
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w := httptest.NewRecorder()

	handler.OptionalAuth(auth, inner).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser != "Optional" {
		t.Fatalf("expected user 'Optional', got %q", gotUser)
	}
}

func TestOptionalAuth_WithoutToken(t *testing.T) {
	auth, _, _, _, _, _, _ := newTestServices(t)

	var gotUser *bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := handler.UserFromContext(r.Context())
		isNil := user == nil
		gotUser = &isNil
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.OptionalAuth(auth, inner).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil || !*gotUser {
		t.Fatal("expected nil user in context for unauthenticated request")
	}
}
