package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const fixture = `{
  "basics": {"name": "Test Person", "label": "Engineer", "signature": "sig.png",
             "altLabels": ["Builder", "Builder"]},
  "work": [{"name": "ACME", "position": "Dev", "highlights": ["one", "two", "three"]}],
  "projects": [{"name": "P", "description": "d", "highlights": ["h1"]}],
  "skills": [{"name": "Go"}],
  "meta": {"rezi_source_id": "keep-me", "synced_tasks": ["old"]},
  "custom_top_level": {"keep": true}
}`

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRoundTripPreservesUnknownTopLevel(t *testing.T) {
	r, err := LoadResume(writeTemp(t, "resume.json", fixture))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.json")
	if err := r.Save(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["custom_top_level"]; !ok {
		t.Error("unknown top-level key was dropped")
	}
	meta := got["meta"].(map[string]any)
	if meta["rezi_source_id"] != "keep-me" {
		t.Errorf("meta.rezi_source_id = %v, want keep-me", meta["rezi_source_id"])
	}
	basics := got["basics"].(map[string]any)
	if basics["signature"] != "sig.png" {
		t.Errorf("basics.signature = %v, want sig.png", basics["signature"])
	}
}

func TestTranslatableDeduplicates(t *testing.T) {
	r, err := LoadResume(writeTemp(t, "resume.json", fixture))
	if err != nil {
		t.Fatal(err)
	}
	refs := r.Translatable()
	seen := map[string]int{}
	for _, ref := range refs {
		seen[ref.EN]++
	}
	if seen["Builder"] != 1 {
		t.Errorf("duplicate string survived: %d occurrences", seen["Builder"])
	}
	if len(refs) != len(seen) {
		t.Errorf("%d refs for %d unique strings", len(refs), len(seen))
	}
}

func TestApplyLanguageFallsBackToEnglish(t *testing.T) {
	r, err := LoadResume(writeTemp(t, "resume.json", fixture))
	if err != nil {
		t.Fatal(err)
	}
	translated, missing := r.ApplyLanguage("de", Cache{})
	if translated != 0 {
		t.Errorf("translated = %d, want 0 with an empty cache", translated)
	}
	if missing == 0 {
		t.Error("expected missing translations to be counted")
	}
	if r.Meta["language"] != "de" {
		t.Errorf("meta.language = %v, want de", r.Meta["language"])
	}
	if r.Basics.Label != "Engineer" {
		t.Errorf("label = %q, want the English fallback", r.Basics.Label)
	}
}
