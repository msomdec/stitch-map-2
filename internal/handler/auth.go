package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	auth         *service.AuthService
	cookieSecure bool
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(auth *service.AuthService, cookieSecure bool) *AuthHandler {
	return &AuthHandler{auth: auth, cookieSecure: cookieSecure}
}

// ShowLogin renders the login page.
func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	view.LoginPage("", "").Render(r.Context(), w)
}

// ShowRegister renders the registration page.
func (h *AuthHandler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	view.RegisterPage("", "", "").Render(r.Context(), w)
}

// HandleRegister processes the registration form submission.
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Error("parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	displayName := r.FormValue("display_name")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	_, err := h.auth.Register(r.Context(), email, displayName, password, confirmPassword)
	if err != nil {
		var errMsg string
		if errors.Is(err, domain.ErrDuplicateEmail) {
			errMsg = "An account with that email already exists."
		} else if errors.Is(err, domain.ErrInvalidInput) {
			errMsg = err.Error()
		} else {
			slog.Error("register user", "error", err)
			errMsg = "An unexpected error occurred. Please try again."
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		view.RegisterPage(errMsg, email, displayName).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HandleLogin processes the login form submission.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Error("parse form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	token, err := h.auth.Login(r.Context(), email, password)
	if err != nil {
		var errMsg string
		if errors.Is(err, domain.ErrUnauthorized) {
			errMsg = "Invalid email or password."
		} else {
			slog.Error("login user", "error", err)
			errMsg = "An unexpected error occurred. Please try again."
		}
		w.WriteHeader(http.StatusUnauthorized)
		view.LoginPage(errMsg, email).Render(r.Context(), w)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleLogout clears the auth cookie and redirects to home.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
