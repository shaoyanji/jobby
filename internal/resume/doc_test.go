package resume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `{
  "basics": {
    "name": "Ada Lovelace",
    "label": "Platform Engineer",
    "altLabels": ["Infrastructure Engineer"],
    "summary": "Builds small reproducible systems.",
    "signature": "https://example.com/sig.png",
    "location": {"city": "Freiburg", "countryCode": "DE"},
    "customField": {"nested": [1, 2, 3]}
  },
  "work": [
    {"name": "O3 Energy", "position": "Platform Engineer", "description": "Fleet.", "highlights": ["Rebuilt CI", "Cut deploys"]},
    {"name": "O3 Energy", "position": "Consultant", "highlights": ["Wrote docs"]}
  ],
  "projects": [
    {"name": "resumectl", "description": "Pipeline.", "roles": ["author"], "highlights": ["Lossless JSON"]}
  ],
  "skills": [{"name": "Nix", "keywords": ["flakes"]}],
  "languages": [{"language": "English", "fluency": "native"}],
  "education": [{"institution": "Uni", "area": "CS", "studyType": "M.Sc.", "courses": ["Compilers"]}],
  "meta": {"language": "en", "rezi_source_id": "abc", "synced_tasks": ["20260101-default"], "count": 2}
}`

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resume.json")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRoundTripKeepsUnknownFields(t *testing.T) {
	path := writeFixture(t)
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetString("work[0].position", "Staff Engineer"); err != nil {
		t.Fatal(err)
	}
	doc.SetMeta("last_synced", "2026-10-24")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	basics := tree["basics"].(map[string]any)
	if _, ok := basics["customField"]; !ok {
		t.Error("basics.customField was dropped")
	}
	if basics["signature"] != "https://example.com/sig.png" {
		t.Errorf("signature = %v", basics["signature"])
	}
	meta := tree["meta"].(map[string]any)
	if meta["rezi_source_id"] != "abc" {
		t.Errorf("meta.rezi_source_id = %v", meta["rezi_source_id"])
	}
	if meta["count"] != float64(2) {
		t.Errorf("meta.count = %v (%T)", meta["count"], meta["count"])
	}
	if meta["last_synced"] != "2026-10-24" {
		t.Errorf("meta.last_synced = %v", meta["last_synced"])
	}
	if got := doc.Root["work"].([]any)[0].(map[string]any)["position"]; got != "Staff Engineer" {
		t.Errorf("position = %v", got)
	}
}

func TestSetStringRefusesMissingPath(t *testing.T) {
	doc, err := Load(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetString("basics.nope", "x"); err == nil {
		t.Fatal("expected an error for a missing path")
	}
	if err := doc.SetString("work[9].position", "x"); err == nil {
		t.Fatal("expected an error for an out-of-range index")
	}
}

func TestRefsCoverTheSchema(t *testing.T) {
	doc, err := Load(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, r := range doc.Refs() {
		paths[r.Path] = r.EN
	}
	for _, want := range []string{
		"basics.label",
		"basics.summary",
		"basics.altLabels[0]",
		"work[0].position",
		"work[0].description",
		"work[0].highlights[1]",
		"projects[0].description",
		"projects[0].roles[0]",
		"projects[0].highlights[0]",
		"skills[0].name",
		"languages[0].language",
		"languages[0].fluency",
		"education[0].area",
		"education[0].studyType",
		"education[0].courses[0]",
	} {
		if _, ok := paths[want]; !ok {
			t.Errorf("missing ref %s (have %v)", want, paths)
		}
	}
	if _, ok := paths["skills[0].keywords[0]"]; ok {
		t.Error("skill keywords must not be translatable")
	}
}

func TestTranslatableDedupesByContent(t *testing.T) {
	doc := &Doc{Root: map[string]any{
		"basics": map[string]any{"label": "Engineer", "summary": "Engineer"},
	}}
	refs := doc.Translatable()
	if len(refs) != 1 {
		t.Fatalf("got %d refs, want 1: %+v", len(refs), refs)
	}
	if refs[0].Path != "basics.label" {
		t.Errorf("kept path %q, want basics.label", refs[0].Path)
	}
}

func TestKeysDisambiguateDuplicates(t *testing.T) {
	doc, err := Load(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	keys := doc.Keys("work")
	if len(keys) != 2 || keys[0] != "O3 Energy" || keys[1] != "O3 Energy (2)" {
		t.Fatalf("keys = %v", keys)
	}
	items := doc.WorkItems()
	if items[1].Detail != "Consultant" {
		t.Errorf("detail = %q", items[1].Detail)
	}
	if got := len(items[0].Highlights); got != 2 {
		t.Errorf("highlights = %d", got)
	}
}

func TestStringsAndMeta(t *testing.T) {
	doc, err := Load(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.MetaStrings("synced_tasks"); len(got) != 1 || got[0] != "20260101-default" {
		t.Errorf("synced_tasks = %v", got)
	}
	doc.SetMetaStrings("synced_tasks", append(doc.MetaStrings("synced_tasks"), "x"))
	if got := doc.MetaStrings("synced_tasks"); len(got) != 2 {
		t.Errorf("synced_tasks after append = %v", got)
	}
	if got := doc.Language(); got != "en" {
		t.Errorf("language = %q", got)
	}
	if labels := doc.Labels(); len(labels) != 2 || !strings.Contains(labels[0], "Platform") {
		t.Errorf("labels = %v", labels)
	}
}
