package layout

import (
	"strings"
	"testing"
)

func TestMeasureUsesTheAFMWidths(t *testing.T) {
	close := func(got, want float64) bool { return got > want-0.005 && got < want+0.005 }
	if got := Measure(" ", 10, helvetica); !close(got, 2.78) {
		t.Errorf("space = %v, want 2.78", got)
	}
	// pdfcpu's bundled bold AFM is narrower than Adobe's: it says 833 for M, and
	// the compiler has to agree with the renderer, not with the spec sheet.
	if got := Measure("M", 10, helveticaBold); !close(got, 8.33) {
		t.Errorf("bold M = %v, want 8.33", got)
	}
	if got := Measure("W", 20, helvetica); !close(got, 18.88) {
		t.Errorf("20pt W = %v, want 18.88", got)
	}
	// Every printable Latin-1 character must have a width; a zero would make the
	// wrapper think the line is shorter than it renders.
	for r := rune(32); r < 256; r++ {
		if winAnsiWidths[helvetica][winAnsiCode(r)] == 0 && r != 127 {
			t.Fatalf("no width for %q (U+%04X)", r, r)
		}
	}
}

func TestWrapFillsTheColumn(t *testing.T) {
	const text = "Built a retrieval layer over internal documents with RAG and MCP tools that survived three model swaps"
	lines := Wrap(text, 200, 10, helvetica)
	if len(lines) < 2 {
		t.Fatalf("expected a wrapped paragraph, got %v", lines)
	}
	for i, line := range lines {
		if w := Measure(line, 10, helvetica); w > 200.5 {
			t.Errorf("line %d is %v wide: %q", i, w, line)
		}
	}
	if strings.Contains(strings.Join(lines, " "), "  ") {
		t.Errorf("double spaces survived the wrap: %v", lines)
	}
}

func TestWrapKeepsExplicitBreaks(t *testing.T) {
	lines := Wrap("one\ntwo", 400, 10, helvetica)
	if len(lines) != 2 || lines[0] != "one" || lines[1] != "two" {
		t.Errorf("lines = %v", lines)
	}
}

func TestWrapSplitsAWordWiderThanTheColumn(t *testing.T) {
	lines := Wrap(strings.Repeat("A", 80), 100, 10, helvetica)
	if len(lines) < 2 {
		t.Fatalf("lines = %v", lines)
	}
	for _, line := range lines {
		if Measure(line, 10, helvetica) > 100.5 {
			t.Errorf("line too wide: %q", line)
		}
	}
}

func TestMeasureIgnoresUnsupportedRunes(t *testing.T) {
	if Measure("→", 10, helvetica) != Measure("?", 10, helvetica) {
		t.Error("an arrow should measure like the fallback glyph")
	}
	if got := Sanitize("a→b…c"); got != "a->b...c" {
		t.Errorf("sanitize = %q", got)
	}
	if got := Sanitize("Straße – Profil"); got != "Straße – Profil" {
		t.Errorf("german text must survive: %q", got)
	}
}
