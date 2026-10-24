package lens

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shaoyanji/jobby/internal/resume"
)

const resumeJSON = `{
  "basics": {"label": "Programmer", "summary": "Generalist."},
  "work": [
    {"name": "Bountystash", "position": "Founder", "highlights": [
      "Built a RAG pipeline with MCP tools and eval harnesses",
      "Ran the storage backend on Postgres",
      "Shipped Go and Python services"
    ]},
    {"name": "O3 Energy", "position": "Platform Engineer", "highlights": [
      "Modelled PPA and weather-derivative exposure",
      "Rebuilt the Nix fleet"
    ]}
  ],
  "projects": [
    {"name": "Swirl Engine", "highlights": ["Risk engine in Rust", "Interactive dashboard"]}
  ],
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

func aiLens() Lens {
	return Lens{
		ID:            "ai",
		Name:          "AI systems",
		Label:         "Software Engineer — AI systems, retrieval, evaluation",
		Keywords:      []string{"RAG", "MCP", "eval", "Go", "Python"},
		MaxHighlights: 2,
	}
}

func TestSelectionUsesKeywordsWithWordBoundaries(t *testing.T) {
	doc := loadDoc(t)
	work, _ := aiLens().Selection(doc)

	// "storage" contains "rag" as a substring; the word boundary must not match.
	if got := work["Bountystash"]; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("Bountystash selection = %v, want [0 2] (RAG/MCP/eval + Go/Python)", got)
	}
	// No keyword hit means keep everything rather than silently emptying a job.
	if got := work["O3 Energy"]; len(got) != 2 {
		t.Errorf("O3 Energy selection = %v, want all bullets", got)
	}
}

func TestExplicitSelectionWinsOverKeywords(t *testing.T) {
	doc := loadDoc(t)
	l := aiLens()
	l.Work = map[string][]int{"Bountystash": {1}}
	work, _ := l.Selection(doc)
	if got := work["Bountystash"]; len(got) != 1 || got[0] != 1 {
		t.Errorf("explicit selection = %v", got)
	}
}

func TestExplicitEmptySelectionMeansKeepNone(t *testing.T) {
	doc := loadDoc(t)
	l := aiLens()
	l.Work = map[string][]int{"O3 Energy": {}}
	work, _ := l.Selection(doc)
	if idx, ok := work["O3 Energy"]; !ok || len(idx) != 0 {
		t.Errorf("empty explicit selection = %v (ok=%v)", idx, ok)
	}
}

func TestIdentityShiftsLabelSummaryAndTitles(t *testing.T) {
	doc := loadDoc(t)
	Identity(doc, Lens{
		Label:   "Senior Quantitative Risk & Energy Systems Architect",
		Summary: "Energy and risk.",
		Titles:  map[string]string{"Bountystash": "Founder & Principal Risk Architect"},
	})
	if doc.PrimaryLabel() != "Senior Quantitative Risk & Energy Systems Architect" {
		t.Errorf("label = %q", doc.PrimaryLabel())
	}
	summary, _ := doc.GetString("basics.summary")
	if summary != "Energy and risk." {
		t.Errorf("summary = %q", summary)
	}
	items := doc.WorkItems()
	if items[0].Detail != "Founder & Principal Risk Architect" {
		t.Errorf("title override = %q", items[0].Detail)
	}
	// Identity must not touch bullets: selection happens once, later.
	if items[0].Highlights[0] == "" || len(items[0].Highlights) != 3 {
		t.Errorf("identity trimmed bullets: %v", items[0].Highlights)
	}
}

func TestLoadAndResolution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lenses.json")
	body := `{"version":1,"active":"ai","lenses":{
	  "ai":{"name":"AI systems","label":"AI"},
	  "energy":{"name":"Energy","label":"Energy"}
	}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if l, ok := f.Get(""); !ok || l.ID != "ai" {
		t.Fatalf("active lens = %+v", l)
	}
	if _, ok := f.Get("nope"); ok {
		t.Error("unknown lens id resolved")
	}
	if ids := f.IDs(); len(ids) != 2 || ids[0] != "ai" {
		t.Errorf("ids = %v", ids)
	}
	if opts := f.Options(); len(opts) != 2 || opts[0] != "AI systems (ai)" {
		t.Errorf("options = %v", opts)
	}

	empty, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := empty.Get(""); ok {
		t.Error("empty lens file resolved a lens")
	}
}
