package service

import (
	"context"
	"fmt"
	"time"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// WorkSessionService handles work session lifecycle and navigation.
type WorkSessionService struct {
	sessions domain.WorkSessionRepository
	patterns domain.PatternRepository
}

// NewWorkSessionService creates a new WorkSessionService.
func NewWorkSessionService(sessions domain.WorkSessionRepository, patterns domain.PatternRepository) *WorkSessionService {
	return &WorkSessionService{sessions: sessions, patterns: patterns}
}

// Start creates a new active work session for the given pattern.
func (s *WorkSessionService) Start(ctx context.Context, userID, patternID int64) (*domain.WorkSession, error) {
	// Verify the pattern exists and belongs to the user.
	pattern, err := s.patterns.GetByID(ctx, patternID)
	if err != nil {
		return nil, fmt.Errorf("get pattern: %w", err)
	}
	if pattern.UserID != userID {
		return nil, domain.ErrUnauthorized
	}
	if len(pattern.InstructionGroups) == 0 {
		return nil, fmt.Errorf("%w: pattern has no instruction groups", domain.ErrInvalidInput)
	}

	session := &domain.WorkSession{
		PatternID: patternID,
		UserID:    userID,
		Status:    domain.SessionStatusActive,
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

// GetByID returns a work session by ID.
func (s *WorkSessionService) GetByID(ctx context.Context, id int64) (*domain.WorkSession, error) {
	return s.sessions.GetByID(ctx, id)
}

// GetActiveByUser returns all active/paused sessions for a user.
func (s *WorkSessionService) GetActiveByUser(ctx context.Context, userID int64) ([]domain.WorkSession, error) {
	return s.sessions.GetActiveByUser(ctx, userID)
}

// GetCompletedByUser returns completed sessions for a user with pagination.
func (s *WorkSessionService) GetCompletedByUser(ctx context.Context, userID int64, limit, offset int) ([]domain.WorkSession, error) {
	return s.sessions.GetCompletedByUser(ctx, userID, limit, offset)
}

// CountCompletedByUser returns the total number of completed sessions for a user.
func (s *WorkSessionService) CountCompletedByUser(ctx context.Context, userID int64) (int, error) {
	return s.sessions.CountCompletedByUser(ctx, userID)
}

// Pause pauses an active session.
func (s *WorkSessionService) Pause(ctx context.Context, session *domain.WorkSession) error {
	if session.Status != domain.SessionStatusActive {
		return fmt.Errorf("%w: session is not active", domain.ErrInvalidInput)
	}
	session.Status = domain.SessionStatusPaused
	return s.sessions.Update(ctx, session)
}

// Resume resumes a paused session.
func (s *WorkSessionService) Resume(ctx context.Context, session *domain.WorkSession) error {
	if session.Status != domain.SessionStatusPaused {
		return fmt.Errorf("%w: session is not paused", domain.ErrInvalidInput)
	}
	session.Status = domain.SessionStatusActive
	return s.sessions.Update(ctx, session)
}

// Abandon deletes a work session.
func (s *WorkSessionService) Abandon(ctx context.Context, id int64) error {
	return s.sessions.Delete(ctx, id)
}

// NavigateForward advances the session position by one stitch.
// Returns true if the pattern is now completed.
func NavigateForward(session *domain.WorkSession, pattern *domain.Pattern) bool {
	groups := pattern.InstructionGroups
	if len(groups) == 0 {
		return true
	}

	gi := session.CurrentGroupIndex
	if gi >= len(groups) {
		return true
	}

	group := &groups[gi]
	entries := group.StitchEntries
	if len(entries) == 0 {
		// Empty group — skip to next group.
		return advanceGroup(session, pattern)
	}

	ei := session.CurrentStitchIndex
	if ei >= len(entries) {
		return advanceGroup(session, pattern)
	}

	entry := &entries[ei]

	// Advance within the stitch count (e.g., "sc 6" means 6 individual stitches).
	session.CurrentStitchCount++
	if session.CurrentStitchCount < entry.Count {
		return false
	}

	// Count exhausted — advance stitch repeat.
	session.CurrentStitchCount = 0
	session.CurrentStitchRepeat++
	if session.CurrentStitchRepeat < entry.RepeatCount {
		return false
	}

	// Repeats exhausted — advance to next stitch entry.
	session.CurrentStitchRepeat = 0
	session.CurrentStitchIndex++
	if session.CurrentStitchIndex < len(entries) {
		return false
	}

	// All entries exhausted — advance group repeat.
	return advanceGroupRepeat(session, pattern)
}

// advanceGroupRepeat handles advancing the group repeat counter.
func advanceGroupRepeat(session *domain.WorkSession, pattern *domain.Pattern) bool {
	group := &pattern.InstructionGroups[session.CurrentGroupIndex]

	session.CurrentStitchIndex = 0
	session.CurrentGroupRepeat++
	if session.CurrentGroupRepeat < group.RepeatCount {
		return false
	}

	// Group repeats exhausted — advance to next group.
	return advanceGroup(session, pattern)
}

// advanceGroup moves to the next instruction group.
func advanceGroup(session *domain.WorkSession, pattern *domain.Pattern) bool {
	session.CurrentGroupIndex++
	session.CurrentGroupRepeat = 0
	session.CurrentStitchIndex = 0
	session.CurrentStitchRepeat = 0
	session.CurrentStitchCount = 0

	if session.CurrentGroupIndex >= len(pattern.InstructionGroups) {
		return true // Pattern completed.
	}

	// Skip empty groups.
	group := &pattern.InstructionGroups[session.CurrentGroupIndex]
	if len(group.StitchEntries) == 0 {
		return advanceGroup(session, pattern)
	}

	return false
}

// NavigateBackward retreats the session position by one stitch.
// Returns false if already at the beginning (no-op).
func NavigateBackward(session *domain.WorkSession, pattern *domain.Pattern) bool {
	if session.CurrentGroupIndex == 0 &&
		session.CurrentGroupRepeat == 0 &&
		session.CurrentStitchIndex == 0 &&
		session.CurrentStitchRepeat == 0 &&
		session.CurrentStitchCount == 0 {
		return false // Already at the beginning.
	}

	// Try to retreat within the stitch count.
	if session.CurrentStitchCount > 0 {
		session.CurrentStitchCount--
		return true
	}

	// Try to retreat within the stitch repeat.
	if session.CurrentStitchRepeat > 0 {
		session.CurrentStitchRepeat--
		entry := &pattern.InstructionGroups[session.CurrentGroupIndex].StitchEntries[session.CurrentStitchIndex]
		session.CurrentStitchCount = entry.Count - 1
		return true
	}

	// Try to retreat to the previous stitch entry.
	if session.CurrentStitchIndex > 0 {
		session.CurrentStitchIndex--
		entry := &pattern.InstructionGroups[session.CurrentGroupIndex].StitchEntries[session.CurrentStitchIndex]
		session.CurrentStitchRepeat = entry.RepeatCount - 1
		session.CurrentStitchCount = entry.Count - 1
		return true
	}

	// Try to retreat within the group repeat.
	if session.CurrentGroupRepeat > 0 {
		session.CurrentGroupRepeat--
		group := &pattern.InstructionGroups[session.CurrentGroupIndex]
		lastEntry := &group.StitchEntries[len(group.StitchEntries)-1]
		session.CurrentStitchIndex = len(group.StitchEntries) - 1
		session.CurrentStitchRepeat = lastEntry.RepeatCount - 1
		session.CurrentStitchCount = lastEntry.Count - 1
		return true
	}

	// Retreat to the previous group.
	return retreatToPreviousGroup(session, pattern)
}

// retreatToPreviousGroup moves to the last stitch of the previous non-empty group.
func retreatToPreviousGroup(session *domain.WorkSession, pattern *domain.Pattern) bool {
	for session.CurrentGroupIndex > 0 {
		session.CurrentGroupIndex--
		group := &pattern.InstructionGroups[session.CurrentGroupIndex]
		if len(group.StitchEntries) == 0 {
			continue // Skip empty groups.
		}
		session.CurrentGroupRepeat = group.RepeatCount - 1
		lastEntry := &group.StitchEntries[len(group.StitchEntries)-1]
		session.CurrentStitchIndex = len(group.StitchEntries) - 1
		session.CurrentStitchRepeat = lastEntry.RepeatCount - 1
		session.CurrentStitchCount = lastEntry.Count - 1
		return true
	}
	return false
}

// AdvanceSession handles a forward navigation request, updating the session state.
func (s *WorkSessionService) AdvanceSession(ctx context.Context, session *domain.WorkSession, pattern *domain.Pattern) (bool, error) {
	completed := NavigateForward(session, pattern)
	if completed {
		session.Status = domain.SessionStatusCompleted
		now := time.Now().UTC()
		session.CompletedAt = &now
	}
	if err := s.sessions.Update(ctx, session); err != nil {
		return false, fmt.Errorf("update session: %w", err)
	}
	return completed, nil
}

// RetreatSession handles a backward navigation request, updating the session state.
func (s *WorkSessionService) RetreatSession(ctx context.Context, session *domain.WorkSession, pattern *domain.Pattern) error {
	NavigateBackward(session, pattern)
	return s.sessions.Update(ctx, session)
}

// SessionProgress computes the overall progress of a session through a pattern.
type SessionProgress struct {
	CompletedStitches int
	TotalStitches     int
	Percentage        float64
	GroupLabel        string
	GroupRepeatInfo   string // e.g., "Repeat 2 of 4"
	CurrentAbbr       string // Current stitch abbreviation
	CurrentName       string // Current stitch name
	PrevAbbr          string // Previous stitch abbreviation (empty if at start)
	NextAbbr          string // Next stitch abbreviation (empty if at end)
	Groups            []GroupProgress
}

// GroupProgress tracks progress for an individual instruction group.
type GroupProgress struct {
	Label            string
	RepeatCount      int
	CurrentRepeat    int    // 1-based, only meaningful when Status == "current"
	Status           string // "completed", "current", "upcoming"
	CompletedInGroup int
	TotalInGroup     int
}

// ComputeProgress calculates the current progress through a pattern.
func ComputeProgress(session *domain.WorkSession, pattern *domain.Pattern) SessionProgress {
	lookup := buildPatternStitchLookup(pattern.PatternStitches)
	nameLookup := buildPatternStitchNameLookup(pattern.PatternStitches)
	total := StitchCount(pattern)

	completed := 0
	groups := pattern.InstructionGroups

	// Count completed stitches up to the current position.
	for gi := range groups {
		group := &groups[gi]
		groupTotal := GroupStitchCount(group)

		if gi < session.CurrentGroupIndex {
			completed += groupTotal * group.RepeatCount
			continue
		}
		if gi > session.CurrentGroupIndex {
			break
		}

		// Current group: count completed repeats.
		completed += groupTotal * session.CurrentGroupRepeat

		// Count completed entries in the current repeat.
		for ei := 0; ei < len(group.StitchEntries); ei++ {
			entry := &group.StitchEntries[ei]
			entryTotal := entry.Count * entry.RepeatCount

			if ei < session.CurrentStitchIndex {
				completed += entryTotal
				continue
			}
			if ei > session.CurrentStitchIndex {
				break
			}

			// Current entry: count completed repeats.
			completed += entry.Count * session.CurrentStitchRepeat
			// Count completed stitches in the current repeat.
			completed += session.CurrentStitchCount
		}
	}

	progress := SessionProgress{
		CompletedStitches: completed,
		TotalStitches:     total,
	}

	if total > 0 {
		progress.Percentage = float64(completed) / float64(total) * 100
	}

	// Current group info.
	if session.CurrentGroupIndex < len(groups) {
		group := &groups[session.CurrentGroupIndex]
		progress.GroupLabel = group.Label
		if group.RepeatCount > 1 {
			progress.GroupRepeatInfo = fmt.Sprintf("Repeat %d of %d", session.CurrentGroupRepeat+1, group.RepeatCount)
		}

		// Current stitch info.
		if session.CurrentStitchIndex < len(group.StitchEntries) {
			entry := &group.StitchEntries[session.CurrentStitchIndex]
			progress.CurrentAbbr = lookup[entry.PatternStitchID]
			progress.CurrentName = nameLookup[entry.PatternStitchID]
		}

		// Previous stitch info.
		progress.PrevAbbr = getPrevStitchAbbr(session, pattern, lookup)
		// Next stitch info.
		progress.NextAbbr = getNextStitchAbbr(session, pattern, lookup)
	}

	// Build per-group progress.
	progress.Groups = make([]GroupProgress, len(groups))
	for gi := range groups {
		group := &groups[gi]
		singleRepeatCount := GroupStitchCount(group)
		totalInGroup := singleRepeatCount * group.RepeatCount

		gp := GroupProgress{
			Label:        group.Label,
			RepeatCount:  group.RepeatCount,
			TotalInGroup: totalInGroup,
		}

		if gi < session.CurrentGroupIndex {
			gp.Status = "completed"
			gp.CompletedInGroup = totalInGroup
		} else if gi == session.CurrentGroupIndex {
			gp.Status = "current"
			gp.CurrentRepeat = session.CurrentGroupRepeat + 1 // 1-based
			// Completed stitches within this group: full repeats + partial current repeat.
			completedInGroup := singleRepeatCount * session.CurrentGroupRepeat
			for ei := 0; ei < len(group.StitchEntries); ei++ {
				entry := &group.StitchEntries[ei]
				entryTotal := entry.Count * entry.RepeatCount
				if ei < session.CurrentStitchIndex {
					completedInGroup += entryTotal
				} else if ei == session.CurrentStitchIndex {
					completedInGroup += entry.Count * session.CurrentStitchRepeat
					completedInGroup += session.CurrentStitchCount
				}
			}
			gp.CompletedInGroup = completedInGroup
		} else {
			gp.Status = "upcoming"
			gp.CompletedInGroup = 0
		}

		progress.Groups[gi] = gp
	}

	return progress
}

func buildPatternStitchNameLookup(stitches []domain.PatternStitch) map[int64]string {
	lookup := make(map[int64]string, len(stitches))
	for _, s := range stitches {
		lookup[s.ID] = s.Name
	}
	return lookup
}

// getPrevStitchAbbr returns the abbreviation of the stitch that would come before
// the current position, or empty if at the start.
func getPrevStitchAbbr(session *domain.WorkSession, pattern *domain.Pattern, lookup map[int64]string) string {
	// If we can retreat within the same entry, it's the same stitch.
	if session.CurrentStitchCount > 0 || session.CurrentStitchRepeat > 0 {
		entry := &pattern.InstructionGroups[session.CurrentGroupIndex].StitchEntries[session.CurrentStitchIndex]
		return lookup[entry.PatternStitchID]
	}

	// Check previous entry in the same group repeat.
	if session.CurrentStitchIndex > 0 {
		entry := &pattern.InstructionGroups[session.CurrentGroupIndex].StitchEntries[session.CurrentStitchIndex-1]
		return lookup[entry.PatternStitchID]
	}

	// Check last entry of the previous group repeat.
	if session.CurrentGroupRepeat > 0 {
		group := &pattern.InstructionGroups[session.CurrentGroupIndex]
		if len(group.StitchEntries) > 0 {
			entry := &group.StitchEntries[len(group.StitchEntries)-1]
			return lookup[entry.PatternStitchID]
		}
	}

	// Check previous group.
	for gi := session.CurrentGroupIndex - 1; gi >= 0; gi-- {
		group := &pattern.InstructionGroups[gi]
		if len(group.StitchEntries) > 0 {
			entry := &group.StitchEntries[len(group.StitchEntries)-1]
			return lookup[entry.PatternStitchID]
		}
	}

	return ""
}

// getNextStitchAbbr returns the abbreviation of the stitch that would come after
// the current position, or empty if at the end.
func getNextStitchAbbr(session *domain.WorkSession, pattern *domain.Pattern, lookup map[int64]string) string {
	gi := session.CurrentGroupIndex
	if gi >= len(pattern.InstructionGroups) {
		return ""
	}

	group := &pattern.InstructionGroups[gi]
	ei := session.CurrentStitchIndex
	if ei >= len(group.StitchEntries) {
		return ""
	}

	entry := &group.StitchEntries[ei]

	// Check if there are more stitches in the current count.
	if session.CurrentStitchCount+1 < entry.Count {
		return lookup[entry.PatternStitchID]
	}

	// Check if there are more repeats of the current entry.
	if session.CurrentStitchRepeat+1 < entry.RepeatCount {
		return lookup[entry.PatternStitchID]
	}

	// Check next entry in the group.
	if ei+1 < len(group.StitchEntries) {
		return lookup[group.StitchEntries[ei+1].PatternStitchID]
	}

	// Check next group repeat.
	if session.CurrentGroupRepeat+1 < group.RepeatCount && len(group.StitchEntries) > 0 {
		return lookup[group.StitchEntries[0].PatternStitchID]
	}

	// Check next group.
	for ngi := gi + 1; ngi < len(pattern.InstructionGroups); ngi++ {
		nextGroup := &pattern.InstructionGroups[ngi]
		if len(nextGroup.StitchEntries) > 0 {
			return lookup[nextGroup.StitchEntries[0].PatternStitchID]
		}
	}

	return ""
}
