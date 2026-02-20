package handler

import (
	"context"
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext extracts the authenticated user from the request context.
// Returns nil if no user is authenticated.
func UserFromContext(ctx context.Context) *domain.User {
	user, _ := ctx.Value(userContextKey).(*domain.User)
	return user
}

// RequireAuth is middleware that protects routes requiring authentication.
// It reads the auth_token cookie, validates the JWT, loads the user from DB,
// and injects it into the request context. Returns 401 for unauthenticated requests.
func RequireAuth(auth *service.AuthService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := authenticateRequest(r, auth)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that attempts to authenticate but does not block
// unauthenticated requests. If a valid token is present, the user is injected
// into context; otherwise the request proceeds without a user.
func OptionalAuth(auth *service.AuthService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := authenticateRequest(r, auth)
		if err == nil && user != nil {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func authenticateRequest(r *http.Request, auth *service.AuthService) (*domain.User, error) {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil, err
	}

	userID, err := auth.ValidateToken(cookie.Value)
	if err != nil {
		return nil, err
	}

	user, err := auth.GetUserByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}
