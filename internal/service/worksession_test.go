package service

import (
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// simplePattern creates a pattern with a single group containing a single stitch entry.
// e.g., "Round 1: 6 sc"
func simplePattern() *domain.Pattern {
	return &domain.Pattern{
		PatternStitches: []domain.PatternStitch{
			{ID: 1, Abbreviation: "sc", Name: "Single Crochet"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 6, RepeatCount: 1},
				},
			},
		},
	}
}

// multiEntryPattern creates a pattern with one group and multiple stitch entries.
// "Round 1: MR, 6 sc" (MR count=1, sc count=6)
func multiEntryPattern() *domain.Pattern {
	return &domain.Pattern{
		PatternStitches: []domain.PatternStitch{
			{ID: 1, Abbreviation: "sc", Name: "Single Crochet"},
			{ID: 4, Abbreviation: "MR", Name: "Magic Ring"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 4, Count: 1, RepeatCount: 1}, // MR
					{PatternStitchID: 1, Count: 6, RepeatCount: 1}, // 6 sc
				},
			},
		},
	}
}

// repeatEntryPattern creates a pattern with stitch repeats.
// "Round 2: *inc, repeat from * 6 times"
func repeatEntryPattern() *domain.Pattern {
	return &domain.Pattern{
		PatternStitches: []domain.PatternStitch{
			{ID: 5, Abbreviation: "inc", Name: "Increase"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 2",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 5, Count: 1, RepeatCount: 6}, // inc ×6
				},
			},
		},
	}
}

// groupRepeatPattern creates a pattern with group repeats.
// "Rounds 3-5 (×3): 2 sc"
func groupRepeatPattern() *domain.Pattern {
	return &domain.Pattern{
		PatternStitches: []domain.PatternStitch{
			{ID: 1, Abbreviation: "sc", Name: "Single Crochet"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Rounds 3-5",
				RepeatCount: 3,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 2, RepeatCount: 1}, // 2 sc
				},
			},
		},
	}
}

// multiGroupPattern creates a pattern with multiple groups.
func multiGroupPattern() *domain.Pattern {
	return &domain.Pattern{
		PatternStitches: []domain.PatternStitch{
			{ID: 1, Abbreviation: "sc", Name: "Single Crochet"},
			{ID: 5, Abbreviation: "inc", Name: "Increase"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 3, RepeatCount: 1}, // 3 sc
				},
			},
			{
				Label:       "Round 2",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 5, Count: 1, RepeatCount: 3}, // inc ×3
				},
			},
		},
	}
}

// complexPattern creates a realistic amigurumi pattern for end-to-end navigation.
// Round 1: MR, 2 sc (3 stitches)
// Round 2: inc ×2 (2 stitches, 1 repeat each)
// Round 3 (×2): sc 2 (2 stitches per repeat × 2 repeats = 4 stitches)
// Total: 9 stitches
func complexPattern() *domain.Pattern {
	return &domain.Pattern{
		PatternStitches: []domain.PatternStitch{
			{ID: 1, Abbreviation: "sc", Name: "Single Crochet"},
			{ID: 4, Abbreviation: "MR", Name: "Magic Ring"},
			{ID: 5, Abbreviation: "inc", Name: "Increase"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 4, Count: 1, RepeatCount: 1}, // MR
					{PatternStitchID: 1, Count: 2, RepeatCount: 1}, // 2 sc
				},
			},
			{
				Label:       "Round 2",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 5, Count: 1, RepeatCount: 2}, // inc ×2
				},
			},
			{
				Label:       "Round 3",
				RepeatCount: 2,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 2, RepeatCount: 1}, // 2 sc
				},
			},
		},
	}
}

func newSession() *domain.WorkSession {
	return &domain.WorkSession{Status: domain.SessionStatusActive}
}

func TestNavigateForward_SimplePattern(t *testing.T) {
	pattern := simplePattern() // 6 sc
	session := newSession()

	// Navigate through all 6 stitches.
	for i := range 5 {
		completed := NavigateForward(session, pattern)
		if completed {
			t.Fatalf("completed too early at stitch %d", i+1)
		}
		if session.CurrentStitchCount != i+1 {
			t.Fatalf("stitch %d: expected count %d, got %d", i+1, i+1, session.CurrentStitchCount)
		}
	}

	// 6th stitch completes the pattern.
	completed := NavigateForward(session, pattern)
	if !completed {
		t.Fatal("expected pattern to be completed after 6 stitches")
	}
}

func TestNavigateForward_MultipleEntries(t *testing.T) {
	pattern := multiEntryPattern() // MR (1), 6 sc
	session := newSession()

	// First stitch: MR (count=1).
	completed := NavigateForward(session, pattern)
	if completed {
		t.Fatal("completed too early after MR")
	}
	// After MR exhausted (count was 1), should advance to next entry.
	if session.CurrentStitchIndex != 1 {
		t.Fatalf("expected stitch index 1, got %d", session.CurrentStitchIndex)
	}

	// Navigate through 5 more sc stitches (indices 0-4 of count).
	for i := range 5 {
		completed = NavigateForward(session, pattern)
		if completed {
			t.Fatalf("completed too early at sc stitch %d", i+1)
		}
	}

	// 6th sc stitch completes the pattern.
	completed = NavigateForward(session, pattern)
	if !completed {
		t.Fatal("expected completed after all stitches")
	}
}

func TestNavigateForward_StitchRepeats(t *testing.T) {
	pattern := repeatEntryPattern() // inc ×6 (count=1 per repeat)
	session := newSession()

	// Each repeat: advance count from 0->1 (exhausting count=1), then increment repeat.
	for i := range 5 {
		completed := NavigateForward(session, pattern)
		if completed {
			t.Fatalf("completed too early at repeat %d", i+1)
		}
		if session.CurrentStitchRepeat != i+1 {
			t.Fatalf("repeat %d: expected stitch repeat %d, got %d", i+1, i+1, session.CurrentStitchRepeat)
		}
	}

	// 6th repeat completes the pattern.
	completed := NavigateForward(session, pattern)
	if !completed {
		t.Fatal("expected completed after 6 repeats")
	}
}

func TestNavigateForward_GroupRepeats(t *testing.T) {
	pattern := groupRepeatPattern() // 2 sc ×3 groups
	session := newSession()

	// First repeat: 2 stitches.
	NavigateForward(session, pattern) // stitch 1 (count 0->1)
	NavigateForward(session, pattern) // stitch 2 (count 1->exhausted, advance group repeat)

	if session.CurrentGroupRepeat != 1 {
		t.Fatalf("expected group repeat 1, got %d", session.CurrentGroupRepeat)
	}
	if session.CurrentStitchCount != 0 {
		t.Fatalf("expected stitch count 0, got %d", session.CurrentStitchCount)
	}

	// Second repeat: 2 stitches.
	NavigateForward(session, pattern)
	NavigateForward(session, pattern)

	if session.CurrentGroupRepeat != 2 {
		t.Fatalf("expected group repeat 2, got %d", session.CurrentGroupRepeat)
	}

	// Third repeat: 2 stitches.
	NavigateForward(session, pattern)
	completed := NavigateForward(session, pattern)

	if !completed {
		t.Fatal("expected completed after 3 group repeats")
	}
}

func TestNavigateForward_MultipleGroups(t *testing.T) {
	pattern := multiGroupPattern()
	session := newSession()

	// Group 1: 3 sc.
	for i := range 3 {
		completed := NavigateForward(session, pattern)
		if completed {
			t.Fatalf("completed too early at group 1 stitch %d", i+1)
		}
	}

	// Should now be in group 2.
	if session.CurrentGroupIndex != 1 {
		t.Fatalf("expected group index 1, got %d", session.CurrentGroupIndex)
	}

	// Group 2: inc ×3 (count=1 per repeat).
	for i := range 2 {
		completed := NavigateForward(session, pattern)
		if completed {
			t.Fatalf("completed too early at group 2 repeat %d", i+1)
		}
	}

	completed := NavigateForward(session, pattern)
	if !completed {
		t.Fatal("expected completed after all groups")
	}
}

func TestNavigateForward_ComplexPattern(t *testing.T) {
	pattern := complexPattern()
	session := newSession()

	// Total stitches: MR(1) + 2sc(2) + inc×2(2) + 2sc×2groups(4) = 9
	total := StitchCount(pattern)
	if total != 9 {
		t.Fatalf("expected 9 total stitches, got %d", total)
	}

	var completed bool
	count := 0
	for !completed {
		completed = NavigateForward(session, pattern)
		count++
		if count > 100 {
			t.Fatal("infinite loop in navigation")
		}
	}

	if count != 9 {
		t.Fatalf("expected 9 forward navigations, got %d", count)
	}
}

func TestNavigateBackward_AtStart(t *testing.T) {
	session := newSession()
	pattern := simplePattern()

	moved := NavigateBackward(session, pattern)
	if moved {
		t.Fatal("should not be able to go backward at start")
	}
}

func TestNavigateBackward_WithinCount(t *testing.T) {
	pattern := simplePattern() // 6 sc
	session := newSession()

	// Advance 3 stitches.
	NavigateForward(session, pattern) // count -> 1
	NavigateForward(session, pattern) // count -> 2
	NavigateForward(session, pattern) // count -> 3

	if session.CurrentStitchCount != 3 {
		t.Fatalf("expected count 3, got %d", session.CurrentStitchCount)
	}

	// Retreat one.
	moved := NavigateBackward(session, pattern)
	if !moved {
		t.Fatal("expected to move backward")
	}
	if session.CurrentStitchCount != 2 {
		t.Fatalf("expected count 2, got %d", session.CurrentStitchCount)
	}
}

func TestNavigateBackward_AcrossEntries(t *testing.T) {
	pattern := multiEntryPattern() // MR (count=1), 6 sc
	session := newSession()

	// Advance past MR into sc.
	NavigateForward(session, pattern) // MR done, now at sc[0]

	if session.CurrentStitchIndex != 1 || session.CurrentStitchCount != 0 {
		t.Fatalf("expected at entry 1 count 0, got entry %d count %d",
			session.CurrentStitchIndex, session.CurrentStitchCount)
	}

	// Retreat back to MR.
	moved := NavigateBackward(session, pattern)
	if !moved {
		t.Fatal("expected to move backward")
	}
	if session.CurrentStitchIndex != 0 {
		t.Fatalf("expected stitch index 0, got %d", session.CurrentStitchIndex)
	}
	if session.CurrentStitchCount != 0 {
		t.Fatalf("expected stitch count 0, got %d", session.CurrentStitchCount)
	}
}

func TestNavigateBackward_AcrossStitchRepeats(t *testing.T) {
	pattern := repeatEntryPattern() // inc ×6 (count=1)
	session := newSession()

	// Advance 2 repeats (each repeat is count=1).
	NavigateForward(session, pattern) // repeat 0 done -> repeat 1
	NavigateForward(session, pattern) // repeat 1 done -> repeat 2

	if session.CurrentStitchRepeat != 2 {
		t.Fatalf("expected repeat 2, got %d", session.CurrentStitchRepeat)
	}

	// Retreat one step.
	moved := NavigateBackward(session, pattern)
	if !moved {
		t.Fatal("expected to move backward")
	}
	// Should be at repeat 1, count 0 (which is the last count of count=1).
	if session.CurrentStitchRepeat != 1 {
		t.Fatalf("expected repeat 1, got %d", session.CurrentStitchRepeat)
	}
	if session.CurrentStitchCount != 0 {
		t.Fatalf("expected count 0, got %d", session.CurrentStitchCount)
	}
}

func TestNavigateBackward_AcrossGroupRepeats(t *testing.T) {
	pattern := groupRepeatPattern() // 2 sc ×3 groups
	session := newSession()

	// Advance through first group repeat (2 stitches).
	NavigateForward(session, pattern)
	NavigateForward(session, pattern)
	// Now at group repeat 1, stitch 0, count 0.

	if session.CurrentGroupRepeat != 1 {
		t.Fatalf("expected group repeat 1, got %d", session.CurrentGroupRepeat)
	}

	// Retreat back into the first group repeat.
	moved := NavigateBackward(session, pattern)
	if !moved {
		t.Fatal("expected to move backward")
	}
	if session.CurrentGroupRepeat != 0 {
		t.Fatalf("expected group repeat 0, got %d", session.CurrentGroupRepeat)
	}
	// Should be at the last stitch of the first repeat: count = 1 (last of Count=2).
	if session.CurrentStitchCount != 1 {
		t.Fatalf("expected stitch count 1, got %d", session.CurrentStitchCount)
	}
}

func TestNavigateBackward_AcrossGroups(t *testing.T) {
	pattern := multiGroupPattern()
	session := newSession()

	// Advance through all of group 1 (3 sc).
	for range 3 {
		NavigateForward(session, pattern)
	}
	// Now at group 2.
	if session.CurrentGroupIndex != 1 {
		t.Fatalf("expected group 1, got %d", session.CurrentGroupIndex)
	}

	// Retreat back to group 1.
	moved := NavigateBackward(session, pattern)
	if !moved {
		t.Fatal("expected to move backward")
	}
	if session.CurrentGroupIndex != 0 {
		t.Fatalf("expected group 0, got %d", session.CurrentGroupIndex)
	}
	// Should be at the last stitch of group 0: count=2 (last of Count=3).
	if session.CurrentStitchCount != 2 {
		t.Fatalf("expected stitch count 2, got %d", session.CurrentStitchCount)
	}
}

func TestNavigateForwardAndBackward_FullRoundTrip(t *testing.T) {
	pattern := complexPattern()
	session := newSession()

	total := StitchCount(pattern) // 9

	// Navigate all the way forward.
	for i := range total {
		completed := NavigateForward(session, pattern)
		if i < total-1 && completed {
			t.Fatalf("completed too early at stitch %d", i+1)
		}
		if i == total-1 && !completed {
			t.Fatal("expected completed after all stitches")
		}
	}

	// Reset to just before completion (back up one, since completion set the group index past end).
	session.Status = domain.SessionStatusActive
	// Navigate all the way backward from the end position.
	// First need to position at the last valid stitch.
	session2 := newSession()
	for i := 0; i < total-1; i++ {
		NavigateForward(session2, pattern)
	}
	// session2 is at the last stitch position. Navigate backward to the start.
	for i := 0; i < total-1; i++ {
		moved := NavigateBackward(session2, pattern)
		if !moved {
			t.Fatalf("unable to move backward at step %d", i+1)
		}
	}
	// Should be back at the start.
	if session2.CurrentGroupIndex != 0 || session2.CurrentGroupRepeat != 0 ||
		session2.CurrentStitchIndex != 0 || session2.CurrentStitchRepeat != 0 ||
		session2.CurrentStitchCount != 0 {
		t.Fatalf("expected back at start, got gi=%d gr=%d si=%d sr=%d sc=%d",
			session2.CurrentGroupIndex, session2.CurrentGroupRepeat,
			session2.CurrentStitchIndex, session2.CurrentStitchRepeat,
			session2.CurrentStitchCount)
	}

	// One more backward should be a no-op.
	moved := NavigateBackward(session2, pattern)
	if moved {
		t.Fatal("should not be able to move backward at start")
	}
}

func TestComputeProgress_AtStart(t *testing.T) {
	pattern := simplePattern()
	session := newSession()

	progress := ComputeProgress(session, pattern)

	if progress.CompletedStitches != 0 {
		t.Fatalf("expected 0 completed, got %d", progress.CompletedStitches)
	}
	if progress.TotalStitches != 6 {
		t.Fatalf("expected 6 total, got %d", progress.TotalStitches)
	}
	if progress.Percentage != 0 {
		t.Fatalf("expected 0%%, got %.1f%%", progress.Percentage)
	}
}

func TestComputeProgress_Midway(t *testing.T) {
	pattern := simplePattern() // 6 sc
	session := newSession()

	// Advance 3 stitches.
	NavigateForward(session, pattern)
	NavigateForward(session, pattern)
	NavigateForward(session, pattern)

	progress := ComputeProgress(session, pattern)

	if progress.CompletedStitches != 3 {
		t.Fatalf("expected 3 completed, got %d", progress.CompletedStitches)
	}
	if progress.Percentage != 50 {
		t.Fatalf("expected 50%%, got %.1f%%", progress.Percentage)
	}
}

func TestComputeProgress_GroupLabel(t *testing.T) {
	pattern := groupRepeatPattern() // "Rounds 3-5" ×3
	session := newSession()

	progress := ComputeProgress(session, pattern)

	if progress.GroupLabel != "Rounds 3-5" {
		t.Fatalf("expected group label 'Rounds 3-5', got %q", progress.GroupLabel)
	}
	if progress.GroupRepeatInfo != "Repeat 1 of 3" {
		t.Fatalf("expected 'Repeat 1 of 3', got %q", progress.GroupRepeatInfo)
	}
}

func TestNavigateForward_EmptyPattern(t *testing.T) {
	pattern := &domain.Pattern{}
	session := newSession()

	completed := NavigateForward(session, pattern)
	if !completed {
		t.Fatal("empty pattern should be immediately completed")
	}
}

func TestNavigateForward_SingleStitch(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: []domain.PatternStitch{
			{ID: 4, Abbreviation: "MR", Name: "Magic Ring"},
		},
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Start",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 4, Count: 1, RepeatCount: 1}, // MR
				},
			},
		},
	}
	session := newSession()

	completed := NavigateForward(session, pattern)
	if !completed {
		t.Fatal("single stitch pattern should be completed after one forward")
	}
}

func TestComputeProgress_GroupsStatus_AtStart(t *testing.T) {
	pattern := complexPattern() // 3 groups
	session := newSession()

	progress := ComputeProgress(session, pattern)

	if len(progress.Groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(progress.Groups))
	}
	if progress.Groups[0].Status != "current" {
		t.Fatalf("expected group 0 status 'current', got %q", progress.Groups[0].Status)
	}
	if progress.Groups[1].Status != "upcoming" {
		t.Fatalf("expected group 1 status 'upcoming', got %q", progress.Groups[1].Status)
	}
	if progress.Groups[2].Status != "upcoming" {
		t.Fatalf("expected group 2 status 'upcoming', got %q", progress.Groups[2].Status)
	}
	if progress.Groups[1].CompletedInGroup != 0 {
		t.Fatalf("expected upcoming group completed 0, got %d", progress.Groups[1].CompletedInGroup)
	}
}

func TestComputeProgress_GroupsStatus_MiddleGroup(t *testing.T) {
	pattern := complexPattern() // Round 1: MR + 2sc (3), Round 2: inc×2 (2), Round 3 ×2: 2sc (4)
	session := newSession()

	// Advance through all of group 0 (3 stitches) into group 1.
	for range 3 {
		NavigateForward(session, pattern)
	}

	progress := ComputeProgress(session, pattern)

	if progress.Groups[0].Status != "completed" {
		t.Fatalf("expected group 0 status 'completed', got %q", progress.Groups[0].Status)
	}
	if progress.Groups[0].CompletedInGroup != progress.Groups[0].TotalInGroup {
		t.Fatalf("expected completed group 0: %d/%d", progress.Groups[0].CompletedInGroup, progress.Groups[0].TotalInGroup)
	}
	if progress.Groups[1].Status != "current" {
		t.Fatalf("expected group 1 status 'current', got %q", progress.Groups[1].Status)
	}
	if progress.Groups[2].Status != "upcoming" {
		t.Fatalf("expected group 2 status 'upcoming', got %q", progress.Groups[2].Status)
	}
}

func TestComputeProgress_GroupsCompletedInGroup(t *testing.T) {
	pattern := simplePattern() // 6 sc in one group
	session := newSession()

	// Advance 4 of 6 stitches.
	for range 4 {
		NavigateForward(session, pattern)
	}

	progress := ComputeProgress(session, pattern)

	if len(progress.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(progress.Groups))
	}
	g := progress.Groups[0]
	if g.Status != "current" {
		t.Fatalf("expected status 'current', got %q", g.Status)
	}
	if g.CompletedInGroup != 4 {
		t.Fatalf("expected 4 completed in group, got %d", g.CompletedInGroup)
	}
	if g.TotalInGroup != 6 {
		t.Fatalf("expected 6 total in group, got %d", g.TotalInGroup)
	}
}

func TestComputeProgress_GroupsWithRepeats(t *testing.T) {
	pattern := groupRepeatPattern() // "Rounds 3-5" ×3, 2 sc per repeat
	session := newSession()

	// Advance through first repeat (2 stitches) into second repeat.
	NavigateForward(session, pattern)
	NavigateForward(session, pattern)

	progress := ComputeProgress(session, pattern)

	if len(progress.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(progress.Groups))
	}
	g := progress.Groups[0]
	if g.RepeatCount != 3 {
		t.Fatalf("expected repeat count 3, got %d", g.RepeatCount)
	}
	if g.CurrentRepeat != 2 {
		t.Fatalf("expected current repeat 2 (1-based), got %d", g.CurrentRepeat)
	}
	if g.TotalInGroup != 6 {
		t.Fatalf("expected 6 total in group (2×3), got %d", g.TotalInGroup)
	}
	if g.CompletedInGroup != 2 {
		t.Fatalf("expected 2 completed in group, got %d", g.CompletedInGroup)
	}
}
