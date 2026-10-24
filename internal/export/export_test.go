package export

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shaoyanji/jobby/internal/i18n"
)

const resumeJSON = `{
  "basics": {"name": "Ada", "label": "Platform Engineer", "summary": "Builds systems", "signature": "sig.png", "custom": {"keep": true}},
  "work": [{"name": "O3", "position": "Engineer", "highlights": ["One", "Two"]}],
  "projects": [{"name": "resumectl", "highlights": ["Lossless"]}],
  "meta": {"language": "en", "synced_tasks": ["20260101-default"], "rezi_source_id": "abc"}
}`

type fixture struct {
	root string
	req  Request
	out  string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	resumePath := filepath.Join(root, "resume.json")
	if err := os.WriteFile(resumePath, []byte(resumeJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	cache, err := i18n.Load(filepath.Join(root, "i18n_cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	for en, de := range map[string]string{
		"Platform Engineer": "Plattform-Engineer",
		"Builds systems":    "Baut Systeme",
		"Engineer":          "Ingenieur",
		"One":               "Eins",
		"Two":               "Zwei",
		"Lossless":          "Verlustfrei",
	} {
		cache.SetAuto(en, de, "")
	}
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "out", "view.json")
	return fixture{
		root: root,
		out:  out,
		req: Request{
			ResumePath:   resumePath,
			CachePath:    filepath.Join(root, "i18n_cache.json"),
			LoadoutsPath: filepath.Join(root, "loadouts.json"),
			Lang:         "de",
			Profile:      "default",
			SaveLoadouts: true,
		},
	}
}

func TestExportTranslatesAndKeepsUnknownFields(t *testing.T) {
	f := newFixture(t)
	res, err := Run(f.req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Translated < 4 {
		t.Errorf("translated = %d, want >= 4", res.Translated)
	}
	if res.Missing != 0 {
		t.Errorf("missing = %d, want 0", res.Missing)
	}
	if _, err := res.Write(f.out, nil); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(f.out)
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	basics := tree["basics"].(map[string]any)
	if basics["label"] != "Plattform-Engineer" {
		t.Errorf("label = %v", basics["label"])
	}
	if basics["custom"] == nil {
		t.Error("unknown basics field dropped")
	}
	meta := tree["meta"].(map[string]any)
	if meta["language"] != "de" {
		t.Errorf("meta.language = %v", meta["language"])
	}
	if meta["rezi_source_id"] != "abc" {
		t.Errorf("meta.rezi_source_id = %v", meta["rezi_source_id"])
	}

	// The source file must be untouched: export only writes the view.
	src, _ := os.ReadFile(f.req.ResumePath)
	if !bytes.Contains(src, []byte("Platform Engineer")) {
		t.Error("export mutated the source resume")
	}
}

func TestExportAppliesLoadoutAndCreatesProfile(t *testing.T) {
	f := newFixture(t)
	f.req.Profile = "trimmed"
	res, err := Run(f.req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Loadout.Name != "trimmed" {
		t.Fatalf("loadout = %+v", res.Loadout)
	}
	if got := res.Doc.Strings("work[0].highlights"); len(got) != 2 {
		t.Errorf("default profile should keep all bullets, got %v", got)
	}

	// The generated profile is persisted so later builds reuse it.
	raw, err := os.ReadFile(filepath.Join(f.root, "loadouts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"trimmed"`)) {
		t.Errorf("loadout not saved: %s", raw)
	}
}

func TestExportStdout(t *testing.T) {
	f := newFixture(t)
	f.req.Lang = "en"
	res, err := Run(f.req)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	written, err := res.Write("-", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if written != "-" {
		t.Errorf("written = %q", written)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"label": "Platform Engineer"`)) {
		t.Errorf("stdout export = %s", buf.String())
	}
}

func TestExportMissingCacheStillProducesValidJSON(t *testing.T) {
	f := newFixture(t)
	os.Remove(f.req.CachePath)
	res, err := Run(f.req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Missing == 0 {
		t.Error("expected missing translations to be counted")
	}
	if _, err := res.JSON(); err != nil {
		t.Fatal(err)
	}
}
