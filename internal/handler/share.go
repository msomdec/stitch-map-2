package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
	"github.com/msomdec/stitch-map-2/internal/view"
)

// ShareHandler handles pattern sharing HTTP requests.
type ShareHandler struct {
	shares   *service.ShareService
	patterns *service.PatternService
	images   *service.ImageService
	users    domain.UserRepository
}

// NewShareHandler creates a new ShareHandler.
func NewShareHandler(shares *service.ShareService, patterns *service.PatternService, images *service.ImageService, users domain.UserRepository) *ShareHandler {
	return &ShareHandler{shares: shares, patterns: patterns, images: images, users: users}
}

// HandleViewShared renders the shared pattern preview page.
// GET /s/{token}
func (h *ShareHandler) HandleViewShared(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	pattern, err := h.shares.GetPatternByShareToken(r.Context(), user.ID, token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrUnauthorized) {
			w.WriteHeader(http.StatusForbidden)
			view.ErrorPage(http.StatusForbidden, "Access Denied", "This pattern was shared with a different account.").Render(r.Context(), w)
			return
		}
		slog.Error("view shared pattern", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// If the viewer is the pattern owner, redirect to the normal view page.
	if pattern.UserID == user.ID {
		http.Redirect(w, r, "/patterns/"+strconv.FormatInt(pattern.ID, 10), http.StatusSeeOther)
		return
	}

	// Load owner name.
	owner, err := h.users.GetByID(r.Context(), pattern.UserID)
	ownerName := "Unknown"
	if err == nil {
		ownerName = owner.DisplayName
	}

	// Load images for the pattern.
	groupImages, err := h.images.ListByPattern(r.Context(), pattern)
	if err != nil {
		slog.Error("list pattern images for share preview", "error", err)
		groupImages = map[int64][]domain.PatternImage{}
	}

	// Check if the viewer has already saved this pattern.
	alreadySaved := false
	var savedPatternID int64
	sharedPatterns, err := h.patterns.ListSharedWithUser(r.Context(), user.ID)
	if err == nil {
		for _, sp := range sharedPatterns {
			if sp.SharedFromUserID != nil && *sp.SharedFromUserID == pattern.UserID && sp.Name == pattern.Name {
				alreadySaved = true
				savedPatternID = sp.ID
				break
			}
		}
	}

	view.SharedPatternPreviewPage(user.DisplayName, pattern, ownerName, groupImages, alreadySaved, savedPatternID, token).Render(r.Context(), w)
}

// HandleSaveShared saves a shared pattern to the viewer's library.
// POST /s/{token}/save
func (h *ShareHandler) HandleSaveShared(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err := h.shares.SaveSharedPattern(r.Context(), user.ID, token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrUnauthorized) {
			w.WriteHeader(http.StatusForbidden)
			view.ErrorPage(http.StatusForbidden, "Access Denied", "This pattern was shared with a different account.").Render(r.Context(), w)
			return
		}
		if errors.Is(err, domain.ErrAlreadySaved) {
			http.Redirect(w, r, "/patterns", http.StatusSeeOther)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Redirect(w, r, "/s/"+token, http.StatusSeeOther)
			return
		}
		slog.Error("save shared pattern", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/patterns", http.StatusSeeOther)
}

// HandleCreateGlobalShare creates a global share link.
// POST /patterns/{id}/share
func (h *ShareHandler) HandleCreateGlobalShare(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err = h.shares.CreateGlobalShare(r.Context(), user.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("create global share", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/patterns/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// HandleCreateEmailShare creates an email-bound share link.
// POST /patterns/{id}/share/email
func (h *ShareHandler) HandleCreateEmailShare(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	recipientEmail := r.FormValue("recipient_email")

	_, err = h.shares.CreateEmailShare(r.Context(), user.ID, id, recipientEmail)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("create email share", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/patterns/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// HandleRevokeShare revokes a single share link.
// POST /patterns/{id}/share/{shareID}/revoke
func (h *ShareHandler) HandleRevokeShare(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	shareID, err := strconv.ParseInt(r.PathValue("shareID"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.shares.RevokeShareForPattern(r.Context(), user.ID, id, shareID); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("revoke share", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/patterns/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// HandleRevokeAllShares revokes all shares for a pattern.
// POST /patterns/{id}/share/revoke-all
func (h *ShareHandler) HandleRevokeAllShares(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.shares.RevokeAllShares(r.Context(), user.ID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("revoke all shares", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/patterns/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
