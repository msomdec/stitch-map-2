package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	auth *service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// HandleLogin processes a JSON login request.
// POST /api/auth/login
// Request:  {"email":"...","password":"..."}
// Response: {"user": {...}}
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "Invalid email or password.")
			return
		}
		slog.Error("login user", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred. Please try again.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	// Retrieve the user to include in the response.
	userID, _ := h.auth.ValidateToken(token)
	user, err := h.auth.GetUserByID(r.Context(), userID)
	if err != nil {
		slog.Error("get user after login", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": toUserDTO(user),
	})
}

// HandleRegister processes a JSON registration request.
// POST /api/auth/register
// Request:  {"email":"...","displayName":"...","password":"...","confirmPassword":"..."}
// Response: {"user": {...}}
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email           string `json:"email"`
		DisplayName     string `json:"displayName"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	user, err := h.auth.Register(r.Context(), req.Email, req.DisplayName, req.Password, req.ConfirmPassword)
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateEmail) {
			writeError(w, http.StatusConflict, "An account with that email already exists.")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("register user", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred. Please try again.")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user": toUserDTO(user),
	})
}

// HandleLogout clears the auth cookie.
// POST /api/auth/logout
// Response: 204 No Content
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

// HandleMe returns the currently authenticated user.
// GET /api/auth/me
// Response: {"user": {...}} or 401
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": toUserDTO(user),
	})
}
