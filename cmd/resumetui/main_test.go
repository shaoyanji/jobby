package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStampSyncAppendsTask(t *testing.T) {
	r := &Resume{Meta: map[string]any{"synced_tasks": []any{"old"}}}
	id := stampSync(r, "energy-de", "de")
	if !strings.Contains(id, "energy-de-de") {
		t.Errorf("id = %q, want loadout and language in it", id)
	}
	if _, ok := r.Meta["last_synced"]; !ok {
		t.Error("meta.last_synced was not set")
	}
	tasks, ok := r.Meta["synced_tasks"].([]any)
	if !ok || len(tasks) != 2 || tasks[0] != "old" {
		t.Errorf("synced_tasks = %#v", r.Meta["synced_tasks"])
	}
}

func TestRunExportWritesBuildJSON(t *testing.T) {
	dir := t.TempDir()
	resumePath := filepath.Join(dir, "resume.json")
	if err := os.WriteFile(resumePath, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Resume:   resumePath,
		Cache:    filepath.Join(dir, "i18n_cache.json"),
		Loadouts: filepath.Join(dir, "loadouts.json"),
		OutDir:   filepath.Join(dir, "dist"),
	}
	out := filepath.Join(dir, "out.json")
	if err := runExport(cfg, "de", "", out); err != nil {
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
	if got["meta"].(map[string]any)["language"] != "de" {
		t.Errorf("meta.language = %v", got["meta"].(map[string]any)["language"])
	}
	tasks := got["meta"].(map[string]any)["synced_tasks"].([]any)
	if len(tasks) != 2 || tasks[0] != "old" {
		t.Errorf("synced_tasks = %v, want the fixture task plus one append", tasks)
	}
	if !strings.Contains(tasks[1].(string), "default-de") {
		t.Errorf("appended task = %v, want loadout and language", tasks[1])
	}
}
