package workbench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shaoyanji/jobby/internal/capsule"
	"github.com/shaoyanji/jobby/internal/i18n"
)

const resumeJSON = `{
  "basics": {"name": "Shao-yan (Matt) Ji", "label": "Programmer", "email": "a@b.c",
    "summary": "Generalist.", "location": {"city": "Freiburg"}},
  "work": [
    {"name": "Bountystash", "position": "Founder", "startDate": "2023-01", "endDate": "",
     "highlights": ["Built a RAG pipeline with MCP tools", "Ran Postgres", "Shipped Go services"]},
    {"name": "O3 Energy", "position": "Platform Engineer", "startDate": "2022-04", "endDate": "",
     "highlights": ["Modelled PPA exposure", "Rebuilt the Nix fleet"]}
  ],
  "projects": [{"name": "Swirl Engine", "roles": ["author"], "highlights": ["Risk engine in Rust"]}],
  "education": [{"institution": "Uni Freiburg", "area": "CS", "studyType": "M.Sc.", "startDate": "2018-10", "endDate": "2021-09"}],
  "meta": {"language": "en", "file_stem": "Matt_Ji"}
}`

const lensesJSON = `{"version":1,"active":"ai","lenses":{
  "ai":{"name":"AI systems","label":"Software Engineer — AI systems, retrieval, evaluation",
        "titles":{"Bountystash":"Senior AI Software Architect"},
        "keywords":["RAG","MCP","eval","Go"],"max_highlights":2},
  "energy":{"name":"Energy","label":"Senior Quantitative Risk & Energy Systems Architect",
        "titles":{"Bountystash":"Founder & Principal Risk Architect"},
        "keywords":["PPA","weather","risk"],"max_highlights":2,
        "work":{"Bountystash":[2],"O3 Energy":[0]}}
}}`

type harness struct {
	root string
	log  string
}

func newHarness(t *testing.T) harness {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"resume.typ", "cover_letter.typ"} {
		if err := os.WriteFile(filepath.Join(root, "templates", name), []byte("// stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "resume.json"), []byte(resumeJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lenses.json"), []byte(lensesJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls.log")
	writeStub(t, bin, "typst", `#!/bin/sh
echo "typst $*" >> "$STUB_LOG"
last=""
for a in "$@"; do last="$a"; done
printf '%%PDF-1.4 stub\n' > "$last"
`)
	writeStub(t, bin, "pdfcpu", `#!/bin/sh
echo "pdfcpu $*" >> "$STUB_LOG"
case "$1" in
  version) echo "version: v0.16.1" ;;
  info) echo "Page count:           1" ;;
  create) last=""; for a in "$@"; do last="$a"; done; printf '%%PDF-1.4 stub\n' > "$last" ;;
  merge) cp "$4" "$3" ;;
  optimize) last=""; for a in "$@"; do last="$a"; done; cp "$3" "$last" ;;
esac
`)
	t.Setenv("STUB_LOG", log)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return harness{root: root, log: log}
}

func writeStub(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (h harness) calls(t *testing.T) string {
	t.Helper()
	b, _ := os.ReadFile(h.log)
	return string(b)
}

func TestBuildProducesCapsuleCVLetterAndPackage(t *testing.T) {
	h := newHarness(t)
	c := capsule.New("TestGmbH")
	c.Role = "Platform Engineer"
	c.Lens = "ai"
	c.Language = "en"

	rep, err := Build(Options{
		Root:     h.root,
		Capsule:  c,
		Budget:   2,
		Optimize: true,
		Log:      func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("build: %v\ncalls:\n%s", err, h.calls(t))
	}
	if rep.Pages != 1 || !rep.Fitted {
		t.Errorf("pages/fitted = %d/%v", rep.Pages, rep.Fitted)
	}

	for _, p := range []string{rep.CV, rep.Letter, rep.Package, rep.CapsuleDir + "/capsule.json"} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing artifact %s: %v", p, err)
		}
	}

	// The CV JSON carries the lens persona, the title override and the trimmed
	// highlight set.
	raw, err := os.ReadFile(filepath.Join(rep.CapsuleDir, "rendered.json"))
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	basics := view["basics"].(map[string]any)
	if !strings.Contains(basics["label"].(string), "AI systems") {
		t.Errorf("label = %v", basics["label"])
	}
	work := view["work"].([]any)
	first := work[0].(map[string]any)
	if first["position"] != "Senior AI Software Architect" {
		t.Errorf("title = %v", first["position"])
	}
	if got := first["highlights"].([]any); len(got) != 2 {
		t.Errorf("highlights = %v, want 2 keyword matches", got)
	}

	// capsule.json is the provenance record and knows about the build.
	caps, err := capsule.Load(rep.CapsuleDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(caps.Builds) != 1 {
		t.Fatalf("builds = %+v", caps.Builds)
	}
	if caps.Builds[0].Preset == "" || caps.Builds[0].Pages != 1 {
		t.Errorf("build record = %+v", caps.Builds[0])
	}
	if caps.Cover.Opening == "" || len(caps.Cover.Body) == 0 {
		t.Errorf("cover letter was not scaffolded: %+v", caps.Cover)
	}
	// The scaffolded paragraphs come from the selected CV bullets.
	if caps.Cover.Body[0] != "Built a RAG pipeline with MCP tools" {
		t.Errorf("seed paragraph = %q", caps.Cover.Body[0])
	}

	calls := h.calls(t)
	for _, want := range []string{
		"pdfcpu create -q " + rep.Layout + " " + rep.CV,
		"pdfcpu attachments add -q " + rep.CV + " " + rep.CapsuleDir + "/capsule.json " + rep.Layout + " " + rep.CapsuleDir + "/rendered.json",
		"pdfcpu merge -q " + rep.Package,
		"pdfcpu properties add -q " + rep.CV,
		"Company = TestGmbH",
		"Lens = ai",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("missing call %q\nlog:\n%s", want, calls)
		}
	}
}

func TestBuildGermanUsesTheCacheAndMockTranslator(t *testing.T) {
	h := newHarness(t)
	c := capsule.New("TestGmbH")
	c.Role = "Platform Engineer"
	c.Lens = "energy"
	c.Language = "de"

	rep, err := Build(Options{
		Root:       h.root,
		Capsule:    c,
		Lang:       "de",
		Translator: i18n.Mock{},
		Delay:      time.Nanosecond,
		Log:        func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("build: %v\ncalls:\n%s", err, h.calls(t))
	}
	if rep.Translated == 0 {
		t.Error("no strings were translated for a German build")
	}
	if rep.Missing != 0 {
		t.Errorf("missing translations = %d (mock should fill everything active)", rep.Missing)
	}

	raw, err := os.ReadFile(filepath.Join(rep.CapsuleDir, "rendered.json"))
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	basics := view["basics"].(map[string]any)
	if label, _ := basics["label"].(string); !strings.HasPrefix(label, "[de] ") {
		t.Errorf("label was not translated: %v", basics["label"])
	}
	position, _ := view["work"].([]any)[0].(map[string]any)["position"].(string)
	if position != "[de] Founder & Principal Risk Architect" {
		t.Errorf("energy lens title = %q", position)
	}

	// The letter is German on the page but English in the capsule, so a later
	// rebuild is deterministic.
	caps, err := capsule.Load(rep.CapsuleDir)
	if err != nil {
		t.Fatal(err)
	}
	if caps.Cover.SourceLang != "en" {
		t.Errorf("cover source lang = %q, want en", caps.Cover.SourceLang)
	}
	if strings.Contains(caps.Cover.Opening, "[de]") {
		t.Error("capsule stored the translated letter instead of the source")
	}
	// Body slices are shared with the rendered copy; the capsule must keep the
	// English paragraphs, not the German ones.
	if len(caps.Cover.Body) == 0 || strings.Contains(caps.Cover.Body[0], "[de]") {
		t.Errorf("capsule stored translated paragraphs: %v", caps.Cover.Body)
	}
	if caps.Cover.Salutation != "Sehr geehrte Damen und Herren," {
		t.Errorf("salutation did not follow the language: %q", caps.Cover.Salutation)
	}

	// The cache is a flat array and holds the active strings.
	cache, err := i18n.Load(filepath.Join(h.root, "i18n_cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.EntriesView()) == 0 {
		t.Error("translation cache is empty")
	}
}

func TestSwitchingLensReseedsButRepeatingItDoesNot(t *testing.T) {
	h := newHarness(t)
	c := capsule.New("SwitchCo")
	c.Lens = "ai"
	quiet := func(string, ...any) {}

	if _, err := Build(Options{Root: h.root, Capsule: c, Lang: "en", NoCover: true, Log: quiet}); err != nil {
		t.Fatal(err)
	}
	if got := c.Work["Bountystash"]; len(got) != 2 {
		t.Fatalf("ai selection = %v", got)
	}

	// An explicit switch to another lens must re-seed titles and bullets.
	if _, err := Build(Options{Root: h.root, Capsule: c, LensID: "energy", Lang: "en", NoCover: true, Log: quiet}); err != nil {
		t.Fatal(err)
	}
	if c.Label != "Senior Quantitative Risk & Energy Systems Architect" {
		t.Errorf("label after switch = %q", c.Label)
	}
	if c.Titles["Bountystash"] != "Founder & Principal Risk Architect" {
		t.Errorf("title after switch = %q", c.Titles["Bountystash"])
	}
	if got := c.Work["Bountystash"]; len(got) != 1 {
		t.Errorf("energy selection = %v", got)
	}

	// Re-running the same lens must keep manual tailoring.
	c.Work["O3 Energy"] = []int{0}
	if _, err := Build(Options{Root: h.root, Capsule: c, LensID: "energy", Lang: "en", NoCover: true, Log: quiet}); err != nil {
		t.Fatal(err)
	}
	if got := c.Work["O3 Energy"]; len(got) != 1 || got[0] != 0 {
		t.Errorf("repeating the lens reset tailoring: %v", got)
	}
}

func TestBuildRejectsUnknownLens(t *testing.T) {
	h := newHarness(t)
	c := capsule.New("TestGmbH")
	c.Lens = "nope"
	_, err := Build(Options{Root: h.root, Capsule: c, Log: func(string, ...any) {}})
	if err == nil || !strings.Contains(err.Error(), "unknown lens") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildCanSkipTheCoverLetter(t *testing.T) {
	h := newHarness(t)
	c := capsule.New("NoCover GmbH")
	c.Lens = "ai"
	rep, err := Build(Options{Root: h.root, Capsule: c, NoCover: true, Log: func(string, ...any) {}})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Letter != "" || rep.Package != "" {
		t.Errorf("letter/package = %q / %q", rep.Letter, rep.Package)
	}
	if _, err := os.Stat(rep.CV); err != nil {
		t.Errorf("cv missing: %v", err)
	}
}

func TestFormatDate(t *testing.T) {
	d := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	if got := formatDate(d, "de"); got != "24. Oktober 2026" {
		t.Errorf("de date = %q", got)
	}
	if got := formatDate(d, "en"); got != "24 October 2026" {
		t.Errorf("en date = %q", got)
	}
}
