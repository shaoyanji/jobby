package loadout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shaoyanji/jobby/internal/resume"
)

const docJSON = `{
  "basics": {"label": "Platform Engineer", "altLabels": ["AI Platform Engineer"]},
  "work": [
    {"name": "O3 Energy", "highlights": ["One", "Two", "Three"]},
    {"name": "Fresh Co", "highlights": ["Alpha", "Beta"]}
  ],
  "projects": [
    {"name": "resumectl", "highlights": ["Lossless", "Bilingual"]}
  ],
  "meta": {"language": "en"}
}`

func loadDoc(t *testing.T) *resume.Doc {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resume.json")
	if err := os.WriteFile(path, []byte(docJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := resume.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestFullSelectsEverything(t *testing.T) {
	doc := loadDoc(t)
	l := Full(doc, "default")
	if l.Label != "Platform Engineer" {
		t.Errorf("label = %q", l.Label)
	}
	if got := l.Work["O3 Energy"]; len(got) != 3 {
		t.Errorf("work selection = %v", got)
	}
	if got := l.Projects["resumectl"]; len(got) != 2 {
		t.Errorf("project selection = %v", got)
	}
}

func TestApplyTrimsInOrder(t *testing.T) {
	doc := loadDoc(t)
	l := Loadout{
		Name:     "ai",
		Label:    "AI Platform Engineer",
		Work:     map[string][]int{"O3 Energy": {2, 0}},
		Projects: map[string][]int{"resumectl": {1}},
	}
	Apply(doc, l)

	if got := doc.PrimaryLabel(); got != "AI Platform Engineer" {
		t.Errorf("persona = %q", got)
	}
	got := doc.Strings("work[0].highlights")
	if len(got) != 2 || got[0] != "Three" || got[1] != "One" {
		t.Errorf("work highlights = %v", got)
	}
	if got := doc.Strings("projects[0].highlights"); len(got) != 1 || got[0] != "Bilingual" {
		t.Errorf("project highlights = %v", got)
	}
	// A key missing from the loadout keeps every bullet: a new role must not
	// vanish from the CV because the deck was not re-edited.
	if got := doc.Strings("work[1].highlights"); len(got) != 2 {
		t.Errorf("untouched work = %v", got)
	}
}

func TestApplyIgnoresOutOfRangeIndices(t *testing.T) {
	doc := loadDoc(t)
	Apply(doc, Loadout{Name: "x", Work: map[string][]int{"O3 Energy": {0, 99, -1}}})
	if got := doc.Strings("work[0].highlights"); len(got) != 1 || got[0] != "One" {
		t.Errorf("highlights = %v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loadouts.json")
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Names()) != 0 {
		t.Fatalf("fresh file has profiles: %v", f.Names())
	}
	f.Put(Loadout{Name: "ai", Label: "AI", Work: map[string][]int{"A": {1}}})
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Active != "ai" {
		t.Errorf("active = %q", again.Active)
	}
	l, ok := again.Get("ai")
	if !ok || len(l.Work["A"]) != 1 {
		t.Fatalf("reloaded loadout = %+v", l)
	}
	if names := again.Names(); len(names) != 1 || names[0] != "ai" {
		t.Errorf("names = %v", names)
	}
}

func TestEnsureDefaultCreatesOnce(t *testing.T) {
	doc := loadDoc(t)
	f, err := Load(filepath.Join(t.TempDir(), "loadouts.json"))
	if err != nil {
		t.Fatal(err)
	}
	l := f.EnsureDefault(doc, "default")
	if l.Name != "default" {
		t.Fatalf("name = %q", l.Name)
	}
	if _, ok := f.Get("default"); !ok {
		t.Fatal("profile not stored")
	}
	// Second call must not rebuild from the document.
	f.Loadouts["default"] = Loadout{Name: "default", Label: "Custom"}
	if got := f.EnsureDefault(doc, "default").Label; got != "Custom" {
		t.Errorf("EnsureDefault overwrote an existing profile: %q", got)
	}
}

func TestApplyWithoutHighlightsDoesNotInventArrays(t *testing.T) {
	doc := loadDoc(t)
	Apply(doc, Loadout{Name: "x", Work: map[string][]int{"O3 Energy": {}}})
	if got := doc.Strings("work[0].highlights"); len(got) != 0 {
		t.Errorf("empty selection should empty the export, got %v", got)
	}
}
