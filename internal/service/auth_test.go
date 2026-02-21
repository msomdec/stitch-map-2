package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
	"github.com/msomdec/stitch-map-2/internal/service"
)

const testJWTSecret = "test-secret-key-for-unit-tests"

func newTestAuthService(t *testing.T) (*service.AuthService, *sqlite.DB) {
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

	userRepo := db.Users()
	// Use cost 4 for fast tests.
	auth := service.NewAuthService(userRepo, testJWTSecret, 4)
	return auth, db
}

func TestAuthService_Register_Success(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	user, err := auth.Register(ctx, "new@example.com", "New User", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if user.ID == 0 {
		t.Fatal("expected user ID to be set")
	}
	if user.Email != "new@example.com" {
		t.Fatalf("expected email new@example.com, got %s", user.Email)
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "dup@example.com", "User 1", "password123", "password123")
	if err != nil {
		t.Fatalf("first register: %v", err)
	}

	_, err = auth.Register(ctx, "dup@example.com", "User 2", "password456", "password456")
	if !errors.Is(err, domain.ErrDuplicateEmail) {
		t.Fatalf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestAuthService_Register_WeakPassword(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "weak@example.com", "Weak", "short", "short")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAuthService_Register_PasswordMismatch(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "mismatch@example.com", "Mismatch", "password123", "different456")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for password mismatch, got %v", err)
	}
}

func TestAuthService_Register_EmptyFields(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		email    string
		display  string
		password string
	}{
		{"empty email", "", "Name", "password123"},
		{"empty display name", "a@b.com", "", "password123"},
		{"empty password", "a@b.com", "Name", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.Register(ctx, tc.email, tc.display, tc.password, tc.password)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "login@example.com", "Login User", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	token, err := auth.Login(ctx, "login@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "wrongpw@example.com", "User", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err = auth.Login(ctx, "wrongpw@example.com", "wrongpassword")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_Login_UnknownEmail(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth.Login(ctx, "nobody@example.com", "password123")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_JWT_GenerateAndValidate(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	user, err := auth.Register(ctx, "jwt@example.com", "JWT User", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	token, err := auth.Login(ctx, "jwt@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	userID, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if userID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, userID)
	}
}

func TestAuthService_JWT_InvalidToken(t *testing.T) {
	auth, _ := newTestAuthService(t)

	_, err := auth.ValidateToken("not-a-valid-jwt")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_JWT_TamperedToken(t *testing.T) {
	auth, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth.Register(ctx, "tamper@example.com", "Tamper", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	token, err := auth.Login(ctx, "tamper@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Tamper with the token by flipping several characters in the signature.
	tampered := token[:len(token)-5] + "XXXXX"
	_, err = auth.ValidateToken(tampered)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for tampered token, got %v", err)
	}
}

func TestAuthService_JWT_WrongSecret(t *testing.T) {
	auth1, _ := newTestAuthService(t)
	ctx := context.Background()

	_, err := auth1.Register(ctx, "secret@example.com", "Secret", "password123", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	token, err := auth1.Login(ctx, "secret@example.com", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Create a second auth service with a different secret.
	dbPath := filepath.Join(t.TempDir(), "test2.db")
	db2, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("New DB2: %v", err)
	}
	defer db2.Close()
	if err := db2.Migrate(ctx); err != nil {
		t.Fatalf("Migrate DB2: %v", err)
	}
	userRepo2 := db2.Users()
	auth2 := service.NewAuthService(userRepo2, "different-secret", 4)

	_, err = auth2.ValidateToken(token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for wrong secret, got %v", err)
	}
}
