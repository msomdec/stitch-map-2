package service

import (
	"testing"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

func testPatternStitches() []domain.PatternStitch {
	return []domain.PatternStitch{
		{ID: 1, Abbreviation: "sc", Name: "Single Crochet"},
		{ID: 2, Abbreviation: "dc", Name: "Double Crochet"},
		{ID: 3, Abbreviation: "ch", Name: "Chain"},
		{ID: 4, Abbreviation: "MR", Name: "Magic Ring"},
		{ID: 5, Abbreviation: "inc", Name: "Increase"},
		{ID: 6, Abbreviation: "sl st", Name: "Slip Stitch"},
	}
}

func TestRenderPatternText_NilPattern(t *testing.T) {
	result := RenderPatternText(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil pattern, got %q", result)
	}
}

func TestRenderPatternText_EmptyGroups(t *testing.T) {
	pattern := &domain.Pattern{
		InstructionGroups: []domain.InstructionGroup{},
	}
	result := RenderPatternText(pattern)
	if result != "" {
		t.Fatalf("expected empty string for empty groups, got %q", result)
	}
}

func TestRenderPatternText_SimpleGroup(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
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
	result := RenderPatternText(pattern)
	// MR counts as 1 stitch entry (count=1) + 6 sc = 7
	expected := "Round 1: MR, 6 sc (7)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderPatternText_GroupWithRepeats(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
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
	result := RenderPatternText(pattern)
	// inc has count=1, repeat=6, so: *inc, repeat from * 6 times (6)
	expected := "Round 2: *inc, repeat from * 6 times (6)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderPatternText_GroupRepeat(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Rounds 3-5",
				RepeatCount: 3,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 12, RepeatCount: 1}, // 12 sc
				},
			},
		},
	}
	result := RenderPatternText(pattern)
	expected := "Rounds 3-5 (×3): 12 sc (12)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderPatternText_MixedEntries(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 4, Count: 1, RepeatCount: 1},  // MR
					{PatternStitchID: 1, Count: 6, RepeatCount: 1},  // 6 sc
				},
			},
			{
				Label:       "Round 2",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 5, Count: 1, RepeatCount: 6}, // inc ×6
				},
			},
			{
				Label:       "Round 3",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 1, RepeatCount: 6},  // sc ×6
					{PatternStitchID: 5, Count: 1, RepeatCount: 6},  // inc ×6
				},
			},
		},
	}
	result := RenderPatternText(pattern)
	// MR counts as 1 stitch entry (count=1) + 6 sc = 7
	expected := "Round 1: MR, 6 sc (7)\nRound 2: *inc, repeat from * 6 times (6)\nRound 3: *sc, repeat from * 6 times, *inc, repeat from * 6 times (12)"
	if result != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestRenderPatternText_WithExpectedCount(t *testing.T) {
	expected := 12
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:         "Round 2",
				RepeatCount:   1,
				ExpectedCount: &expected,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 5, Count: 1, RepeatCount: 6},
				},
			},
		},
	}
	result := RenderPatternText(pattern)
	want := "Round 2: *inc, repeat from * 6 times (12)"
	if result != want {
		t.Fatalf("expected %q, got %q", want, result)
	}
}

func TestRenderPatternText_WithIntoStitch(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 6, RepeatCount: 1, IntoStitch: "into ring"},
				},
			},
		},
	}
	result := RenderPatternText(pattern)
	expected := "Round 1: 6 sc into ring (6)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderPatternText_SingleStitch(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
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
	result := RenderPatternText(pattern)
	expected := "Start: MR (1)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderPatternText_EmptyGroupEntries(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:         "Setup",
				RepeatCount:   1,
				StitchEntries: []domain.StitchEntry{},
			},
		},
	}
	result := RenderPatternText(pattern)
	expected := "Setup:"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderPatternText_UnknownStitchID(t *testing.T) {
	pattern := &domain.Pattern{
		PatternStitches: testPatternStitches(),
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 999, Count: 3, RepeatCount: 1},
				},
			},
		},
	}
	result := RenderPatternText(pattern)
	expected := "Round 1: 3 ? (3)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderGroupText(t *testing.T) {
	g := &domain.InstructionGroup{
		Label:       "Round 1",
		RepeatCount: 1,
		StitchEntries: []domain.StitchEntry{
			{PatternStitchID: 1, Count: 6, RepeatCount: 1},
		},
	}
	result := RenderGroupText(g, testPatternStitches())
	expected := "Round 1: 6 sc (6)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRenderGroupText_Nil(t *testing.T) {
	result := RenderGroupText(nil, testPatternStitches())
	if result != "" {
		t.Fatalf("expected empty string for nil group, got %q", result)
	}
}

func TestRenderPatternText_ComplexPattern(t *testing.T) {
	// A realistic amigurumi ball pattern.
	ec12 := 12
	pattern := &domain.Pattern{
		Name:            "Amigurumi Ball",
		PatternType:     domain.PatternTypeRound,
		PatternStitches: testPatternStitches(),
		InstructionGroups: []domain.InstructionGroup{
			{
				Label:       "Round 1",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 4, Count: 1, RepeatCount: 1},  // MR
					{PatternStitchID: 1, Count: 6, RepeatCount: 1},  // 6 sc
				},
			},
			{
				Label:         "Round 2",
				RepeatCount:   1,
				ExpectedCount: &ec12,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 5, Count: 1, RepeatCount: 6}, // inc ×6
				},
			},
			{
				Label:       "Rounds 3-5",
				RepeatCount: 3,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 1, Count: 12, RepeatCount: 1}, // 12 sc
				},
			},
			{
				Label:       "Finish",
				RepeatCount: 1,
				StitchEntries: []domain.StitchEntry{
					{PatternStitchID: 6, Count: 1, RepeatCount: 1}, // sl st
				},
			},
		},
	}

	result := RenderPatternText(pattern)
	// MR counts as 1 stitch entry (count=1) + 6 sc = 7
	expected := "Round 1: MR, 6 sc (7)\nRound 2: *inc, repeat from * 6 times (12)\nRounds 3-5 (×3): 12 sc (12)\nFinish: sl st (1)"
	if result != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, result)
	}
}
