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

// WorkSessionHandler handles work session HTTP requests.
type WorkSessionHandler struct {
	sessions *service.WorkSessionService
	patterns *service.PatternService
	stitches *service.StitchService
}

// NewWorkSessionHandler creates a new WorkSessionHandler.
func NewWorkSessionHandler(sessions *service.WorkSessionService, patterns *service.PatternService, stitches *service.StitchService) *WorkSessionHandler {
	return &WorkSessionHandler{sessions: sessions, patterns: patterns, stitches: stitches}
}

// HandleStart starts a new work session for a pattern.
func (h *WorkSessionHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	patternID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	session, err := h.sessions.Start(r.Context(), user.ID, patternID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		slog.Error("start work session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sessions/"+strconv.FormatInt(session.ID, 10), http.StatusSeeOther)
}

// HandleView renders the work session tracker page.
func (h *WorkSessionHandler) HandleView(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	session, err := h.sessions.GetByID(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("get work session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if session.UserID != user.ID {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	pattern, err := h.patterns.GetByID(r.Context(), session.PatternID)
	if err != nil {
		slog.Error("get pattern for session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	allStitches, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stitches for session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	progress := service.ComputeProgress(session, pattern, allStitches)

	view.WorkSessionPage(user.DisplayName, session, pattern, progress).Render(r.Context(), w)
}

// HandleForward advances the session by one stitch.
func (h *WorkSessionHandler) HandleForward(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, pattern, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		handleSessionError(w, err)
		return
	}

	if session.Status != domain.SessionStatusActive {
		http.Redirect(w, r, "/sessions/"+strconv.FormatInt(session.ID, 10), http.StatusSeeOther)
		return
	}

	if _, err := h.sessions.AdvanceSession(r.Context(), session, pattern); err != nil {
		slog.Error("advance session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sessions/"+strconv.FormatInt(session.ID, 10), http.StatusSeeOther)
}

// HandleBackward retreats the session by one stitch.
func (h *WorkSessionHandler) HandleBackward(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, pattern, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		handleSessionError(w, err)
		return
	}

	if session.Status != domain.SessionStatusActive {
		http.Redirect(w, r, "/sessions/"+strconv.FormatInt(session.ID, 10), http.StatusSeeOther)
		return
	}

	if err := h.sessions.RetreatSession(r.Context(), session, pattern); err != nil {
		slog.Error("retreat session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sessions/"+strconv.FormatInt(session.ID, 10), http.StatusSeeOther)
}

// HandlePause pauses an active session.
func (h *WorkSessionHandler) HandlePause(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, _, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		handleSessionError(w, err)
		return
	}

	if err := h.sessions.Pause(r.Context(), session); err != nil {
		slog.Error("pause session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sessions/"+strconv.FormatInt(session.ID, 10), http.StatusSeeOther)
}

// HandleResume resumes a paused session.
func (h *WorkSessionHandler) HandleResume(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session, _, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		handleSessionError(w, err)
		return
	}

	if err := h.sessions.Resume(r.Context(), session); err != nil {
		slog.Error("resume session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sessions/"+strconv.FormatInt(session.ID, 10), http.StatusSeeOther)
}

// HandleAbandon deletes a work session.
func (h *WorkSessionHandler) HandleAbandon(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	session, err := h.sessions.GetByID(r.Context(), sessionID)
	if err != nil {
		handleSessionError(w, err)
		return
	}

	if session.UserID != user.ID {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := h.sessions.Abandon(r.Context(), sessionID); err != nil {
		slog.Error("abandon session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *WorkSessionHandler) loadSessionAndPattern(r *http.Request, userID int64) (*domain.WorkSession, *domain.Pattern, error) {
	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, nil, domain.ErrInvalidInput
	}

	session, err := h.sessions.GetByID(r.Context(), sessionID)
	if err != nil {
		return nil, nil, err
	}

	if session.UserID != userID {
		return nil, nil, domain.ErrUnauthorized
	}

	pattern, err := h.patterns.GetByID(r.Context(), session.PatternID)
	if err != nil {
		return nil, nil, err
	}

	return session, pattern, nil
}

func handleSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	slog.Error("session operation", "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
