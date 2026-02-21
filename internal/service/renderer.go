package service

import (
	"fmt"
	"strings"

	"github.com/msomdec/stitch-map-2/internal/domain"
)

// RenderPatternText renders a pattern as formatted text using standard
// crochet notation. It resolves stitch IDs to abbreviations using the
// provided stitch list.
func RenderPatternText(pattern *domain.Pattern, stitches []domain.Stitch) string {
	if pattern == nil || len(pattern.InstructionGroups) == 0 {
		return ""
	}

	lookup := buildStitchLookup(stitches)
	var lines []string

	for _, g := range pattern.InstructionGroups {
		line := renderGroup(&g, lookup)
		if line != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// RenderGroupText renders a single instruction group as text.
func RenderGroupText(g *domain.InstructionGroup, stitches []domain.Stitch) string {
	if g == nil {
		return ""
	}
	lookup := buildStitchLookup(stitches)
	return renderGroup(g, lookup)
}

func buildStitchLookup(stitches []domain.Stitch) map[int64]string {
	lookup := make(map[int64]string, len(stitches))
	for _, s := range stitches {
		lookup[s.ID] = s.Abbreviation
	}
	return lookup
}

func renderGroup(g *domain.InstructionGroup, lookup map[int64]string) string {
	if len(g.StitchEntries) == 0 {
		return g.Label + ":"
	}

	label := g.Label
	if g.RepeatCount > 1 {
		label = fmt.Sprintf("%s (×%d)", g.Label, g.RepeatCount)
	}

	entries := renderEntries(g.StitchEntries, lookup)
	stitchCount := GroupStitchCount(g)

	if g.ExpectedCount != nil {
		return fmt.Sprintf("%s: %s (%d)", label, entries, *g.ExpectedCount)
	}
	return fmt.Sprintf("%s: %s (%d)", label, entries, stitchCount)
}

func renderEntries(entries []domain.StitchEntry, lookup map[int64]string) string {
	// Check if all entries together form a repeated sequence.
	// Use *..., repeat from * notation for entries with RepeatCount > 1.
	var parts []string

	for _, e := range entries {
		part := renderEntry(&e, lookup)
		if e.RepeatCount > 1 {
			part = fmt.Sprintf("*%s, repeat from * %d times", part, e.RepeatCount)
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, ", ")
}

func renderEntry(e *domain.StitchEntry, lookup map[int64]string) string {
	abbr := lookup[e.StitchID]
	if abbr == "" {
		abbr = "?"
	}

	var sb strings.Builder

	// For count > 1, prefix with the count.
	if e.Count > 1 {
		fmt.Fprintf(&sb, "%d %s", e.Count, abbr)
	} else {
		sb.WriteString(abbr)
	}

	// Append "into" instruction.
	if e.IntoStitch != "" {
		fmt.Fprintf(&sb, " %s", e.IntoStitch)
	}

	return sb.String()
}
