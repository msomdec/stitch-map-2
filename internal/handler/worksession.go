package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
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
// POST /api/patterns/{id}/sessions
// Response: {"session": {...}}
func (h *WorkSessionHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	patternID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid pattern ID.")
		return
	}

	session, err := h.sessions.Start(r.Context(), user.ID, patternID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusNotFound, "Pattern not found.")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("start work session", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"session": toWorkSessionDTO(session),
	})
}

// HandleView returns a work session with its pattern and progress info.
// GET /api/sessions/{id}
// Response: {"session": {...}, "pattern": {...}, "progress": {...}}
func (h *WorkSessionHandler) HandleView(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	session, pattern, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		writeSessionError(w, err)
		return
	}

	allStitches, err := h.stitches.ListAll(r.Context(), user.ID)
	if err != nil {
		slog.Error("list stitches for session", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	progress := service.ComputeProgress(session, pattern, allStitches)

	writeJSON(w, http.StatusOK, map[string]any{
		"session":  toWorkSessionDTO(session),
		"pattern":  toPatternDTO(pattern),
		"progress": toSessionProgressDTO(progress),
	})
}

// HandleForward advances the session by one stitch.
// POST /api/sessions/{id}/next
// Response: {"session": {...}, "pattern": {...}, "progress": {...}}
func (h *WorkSessionHandler) HandleForward(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	session, pattern, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		writeSessionError(w, err)
		return
	}

	if session.Status != domain.SessionStatusActive {
		writeError(w, http.StatusUnprocessableEntity, "Session is not active.")
		return
	}

	if _, err := h.sessions.AdvanceSession(r.Context(), session, pattern); err != nil {
		slog.Error("advance session", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	h.writeSessionResponse(w, r, session, pattern, user.ID)
}

// HandleBackward retreats the session by one stitch.
// POST /api/sessions/{id}/prev
// Response: {"session": {...}, "pattern": {...}, "progress": {...}}
func (h *WorkSessionHandler) HandleBackward(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	session, pattern, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		writeSessionError(w, err)
		return
	}

	if session.Status != domain.SessionStatusActive {
		writeError(w, http.StatusUnprocessableEntity, "Session is not active.")
		return
	}

	if err := h.sessions.RetreatSession(r.Context(), session, pattern); err != nil {
		slog.Error("retreat session", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	h.writeSessionResponse(w, r, session, pattern, user.ID)
}

// HandlePause pauses an active session.
// POST /api/sessions/{id}/pause
// Response: {"session": {...}, "pattern": {...}, "progress": {...}}
func (h *WorkSessionHandler) HandlePause(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	session, pattern, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		writeSessionError(w, err)
		return
	}

	if err := h.sessions.Pause(r.Context(), session); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("pause session", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	h.writeSessionResponse(w, r, session, pattern, user.ID)
}

// HandleResume resumes a paused session.
// POST /api/sessions/{id}/resume
// Response: {"session": {...}, "pattern": {...}, "progress": {...}}
func (h *WorkSessionHandler) HandleResume(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	session, pattern, err := h.loadSessionAndPattern(r, user.ID)
	if err != nil {
		writeSessionError(w, err)
		return
	}

	if err := h.sessions.Resume(r.Context(), session); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("resume session", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	h.writeSessionResponse(w, r, session, pattern, user.ID)
}

// HandleAbandon deletes a work session.
// DELETE /api/sessions/{id}
// Response: 204 No Content
func (h *WorkSessionHandler) HandleAbandon(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized.")
		return
	}

	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid session ID.")
		return
	}

	session, err := h.sessions.GetByID(r.Context(), sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}

	if session.UserID != user.ID {
		writeError(w, http.StatusNotFound, "Session not found.")
		return
	}

	if err := h.sessions.Abandon(r.Context(), sessionID); err != nil {
		slog.Error("abandon session", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeSessionResponse computes progress and writes the standard session JSON response.
func (h *WorkSessionHandler) writeSessionResponse(w http.ResponseWriter, r *http.Request, session *domain.WorkSession, pattern *domain.Pattern, userID int64) {
	allStitches, err := h.stitches.ListAll(r.Context(), userID)
	if err != nil {
		slog.Error("list stitches for session response", "error", err)
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
		return
	}

	progress := service.ComputeProgress(session, pattern, allStitches)

	writeJSON(w, http.StatusOK, map[string]any{
		"session":  toWorkSessionDTO(session),
		"pattern":  toPatternDTO(pattern),
		"progress": toSessionProgressDTO(progress),
	})
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

func writeSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
		writeError(w, http.StatusNotFound, "Session not found.")
		return
	}
	if errors.Is(err, domain.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "Invalid session ID.")
		return
	}
	slog.Error("session operation", "error", err)
	writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
}
