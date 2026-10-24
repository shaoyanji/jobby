package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const resumeJSON = `{
  "basics": {"name": "Ada", "label": "Platform Engineer", "summary": "Builds systems"},
  "work": [{"name": "O3", "position": "Engineer", "highlights": ["One", "Two"]}],
  "meta": {"language": "en", "rezi_source_id": "abc"}
}`

func setupRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "resume.json"), []byte(resumeJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRenderJSONWritesTheView(t *testing.T) {
	root := setupRoot(t)
	out := filepath.Join(root, "dist", "view.json")
	if err := cmdRender([]string{"--root", root, "--lang", "de", "--profile", "default", "--out", out}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	meta := tree["meta"].(map[string]any)
	if meta["language"] != "de" {
		t.Errorf("language = %v", meta["language"])
	}
	if meta["rezi_source_id"] != "abc" {
		t.Errorf("unknown meta key dropped: %v", meta)
	}
	if _, err := os.Stat(filepath.Join(root, "loadouts.json")); err != nil {
		t.Errorf("default profile was not persisted: %v", err)
	}
}

func TestSyncMockThenRenderUsesGerman(t *testing.T) {
	root := setupRoot(t)
	if err := cmdSync([]string{"--root", root, "--mock", "--delay", "1ms"}); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(root, "i18n_cache.json")
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
		t.Fatalf("cache is not a flat array:\n%s", raw)
	}
	if !strings.Contains(string(raw), "[de] Platform Engineer") {
		t.Errorf("mock translation missing:\n%s", raw)
	}

	out := filepath.Join(root, "dist", "de.json")
	if err := cmdRender([]string{"--root", root, "--lang", "de", "--out", out}); err != nil {
		t.Fatal(err)
	}
	view, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(view), "[de] Platform Engineer") {
		t.Errorf("render did not use the cache:\n%s", view)
	}
}

func TestCommandsRejectUnreadableResume(t *testing.T) {
	root := t.TempDir()
	err := cmdRender([]string{"--root", root, "--lang", "en", "--out", "-"})
	if err == nil || !strings.Contains(err.Error(), "read resume") {
		t.Fatalf("err = %v, want a read error", err)
	}
	if err := cmdStats([]string{"--root", root}); err == nil {
		t.Fatal("stats should fail without a resume")
	}
}

func TestUnpackRequiresAnArgument(t *testing.T) {
	if err := cmdUnpack([]string{"--root", t.TempDir()}); err == nil {
		t.Fatal("expected a usage error")
	}
}
