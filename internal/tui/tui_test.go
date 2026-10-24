package tui

import "testing"

func TestPresetFallsBackToAll(t *testing.T) {
	got := preset(nil, 3)
	if len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Errorf("preset(nil, 3) = %v", got)
	}
}

func TestPresetKeepsSavedSelectionAndDropsStaleIndices(t *testing.T) {
	got := preset([]int{2, 7, 0}, 3)
	if len(got) != 2 || got[0] != 2 || got[1] != 0 {
		t.Errorf("preset = %v, want [2 0]", got)
	}
	if got := preset([]int{0}, 0); len(got) != 0 {
		t.Errorf("preset against an empty list = %v", got)
	}
}

func TestClipCollapsesWhitespace(t *testing.T) {
	if got := clip("a\n  b\tc", 20); got != "a b c" {
		t.Errorf("clip = %q", got)
	}
	got := clip("abcdef", 4)
	if len([]rune(got)) != 4 || got[len(got)-3:] != "…" {
		t.Errorf("clip truncation = %q", got)
	}
}
