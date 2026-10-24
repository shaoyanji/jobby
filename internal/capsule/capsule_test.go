package capsule

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shaoyanji/jobby/internal/coverletter"
	"github.com/shaoyanji/jobby/internal/resume"
)

const resumeJSON = `{
  "basics": {"name": "Shao-yan (Matt) Ji", "label": "Programmer"},
  "work": [
    {"name": "Bountystash", "position": "Founder", "highlights": ["One", "Two", "Three"]},
    {"name": "O3 Energy", "position": "Platform Engineer", "highlights": ["Alpha", "Beta"]}
  ],
  "projects": [{"name": "Swirl Engine", "highlights": ["Risk engine", "Dashboard"]}],
  "meta": {"language": "en"}
}`

func loadDoc(t *testing.T) *resume.Doc {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resume.json")
	if err := os.WriteFile(path, []byte(resumeJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := resume.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"TestGmbH":        "testgmbh",
		"O3 Energy":       "o3-energy",
		"  Solar & Wind ": "solar-wind",
		"a.b_c":           "a-b-c",
		"":                "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDirNameAndArtifacts(t *testing.T) {
	c := New("Test GmbH")
	c.CreatedAt = "2026-10-24T08:00:00Z"
	c.FileStem = "Matt_Ji"
	if got := c.DirName(); got != "2026-10-24-test-gmbh" {
		t.Errorf("dir = %q", got)
	}
	if got := c.Dir("/repo"); got != filepath.Join("/repo", "applications", "2026-10-24-test-gmbh") {
		t.Errorf("dir = %q", got)
	}
	if got := c.Artifact("cv"); got != "CV_Matt_Ji.pdf" {
		t.Errorf("cv = %q", got)
	}
	if got := c.Artifact("package"); got != "Application_Package_Matt_Ji.pdf" {
		t.Errorf("package = %q", got)
	}
}

func TestSaveLoadRoundTripAndList(t *testing.T) {
	root := t.TempDir()
	c := New("TestGmbH")
	c.Role = "Platform Engineer"
	c.Language = "de"
	c.Lens = "ai"
	c.Cover = coverletter.Letter{Opening: "Hallo", Body: []string{"a", "b"}, Closing: "Grüße"}
	c.Work = map[string][]int{"Bountystash": {0, 2}}
	c.AddBuild(BuildRecord{At: "2026-10-24T08:00:00Z", Lang: "de", Pages: 2})

	dir, err := c.Save(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "capsule.json")); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Company != "TestGmbH" || loaded.Language != "de" || loaded.Lens != "ai" {
		t.Errorf("loaded = %+v", loaded)
	}
	if len(loaded.Cover.Body) != 2 || loaded.Cover.Body[1] != "b" {
		t.Errorf("cover body = %v", loaded.Cover.Body)
	}
	if len(loaded.Builds) != 1 || loaded.Builds[0].Pages != 2 {
		t.Errorf("builds = %+v", loaded.Builds)
	}

	sums, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 1 || sums[0].Company != "TestGmbH" {
		t.Fatalf("list = %+v", sums)
	}

	found, err := Find(root, "testgmbh")
	if err != nil || found == nil {
		t.Fatalf("find = %v, %v", found, err)
	}
	if found.Role != "Platform Engineer" {
		t.Errorf("found role = %q", found.Role)
	}
	if missing, err := Find(root, "Other Co"); err != nil || missing != nil {
		t.Errorf("find for unknown company = %v, %v", missing, err)
	}
}

func TestApplyTrimsOnceAndOverridesTitles(t *testing.T) {
	doc := loadDoc(t)
	c := New("TestGmbH")
	c.Label = "Quant"
	c.Titles = map[string]string{"Bountystash": "Founder & Principal Risk Architect"}
	c.Work = map[string][]int{"Bountystash": {2, 0}}
	c.Projects = map[string][]int{"Swirl Engine": {1}}

	c.Apply(doc)

	if doc.PrimaryLabel() != "Quant" {
		t.Errorf("label = %q", doc.PrimaryLabel())
	}
	items := doc.WorkItems()
	if items[0].Detail != "Founder & Principal Risk Architect" {
		t.Errorf("title = %q", items[0].Detail)
	}
	if got := doc.Strings("work[0].highlights"); len(got) != 2 || got[0] != "Three" || got[1] != "One" {
		t.Errorf("highlights = %v", got)
	}
	// Keys not in the capsule keep every bullet.
	if got := doc.Strings("work[1].highlights"); len(got) != 2 {
		t.Errorf("untouched work = %v", got)
	}
	if got := doc.Strings("projects[0].highlights"); len(got) != 1 || got[0] != "Dashboard" {
		t.Errorf("project highlights = %v", got)
	}
}

func TestAddBuildKeepsTheLastTwenty(t *testing.T) {
	c := New("X")
	for i := 0; i < 25; i++ {
		c.AddBuild(BuildRecord{At: "t", Lang: "en"})
	}
	if len(c.Builds) != 20 {
		t.Errorf("builds = %d", len(c.Builds))
	}
}

func TestDefaultFileStem(t *testing.T) {
	cases := map[string]string{
		"Shao-yan (Matt) Ji": "Shao-yan_Ji",
		"Ada Lovelace":       "Ada_Lovelace",
		"Cher":               "Cher",
		"":                   "Resume",
	}
	for in, want := range cases {
		if got := DefaultFileStem(in); got != want {
			t.Errorf("DefaultFileStem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadRejectsBrokenJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capsule.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestSaveIsAtomicAndLeavesNoTempFiles(t *testing.T) {
	root := t.TempDir()
	c := New("Atomic Co")
	dir, err := c.Save(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "capsule.json" {
			t.Errorf("unexpected leftover %q", e.Name())
		}
	}
}
