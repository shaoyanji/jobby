package layout

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaoyanji/jobby/internal/coverletter"
	"github.com/shaoyanji/jobby/internal/resume"
)

const fixture = `{
  "basics": {
    "name": "Matt Ji",
    "label": "Senior Engineer",
    "altLabels": ["Risk Architect", "Consultant"],
    "email": "matt@example.com",
    "url": "https://example.com",
    "summary": "Builds retrieval systems and hedging models.",
    "location": {"city": "Freiburg", "countryCode": "DE"}
  },
  "work": [
    {
      "name": "Bountystash",
      "position": "Senior AI Software Architect",
      "startDate": "2023-01",
      "endDate": "today",
      "summary": "Owns an AI platform end to end.",
      "highlights": [
        "Built a retrieval layer over internal documents with RAG and MCP tools",
        "Wrote the evaluation harness that every model change runs through"
      ]
    },
    {
      "name": "O3 Energy",
      "position": "Quantitative Risk Lead",
      "startDate": "2020-03",
      "endDate": "2022-12",
      "highlights": ["Modelled PPA exposure for a 40 MW solar portfolio"]
    }
  ],
  "projects": [
    {"name": "Swirl Engine", "description": "Weather derivatives pricing", "highlights": ["Vectorised Monte Carlo in Go"]}
  ],
  "skills": [{"name": "Distributed Systems", "keywords": ["Go", "Rust", "NATS"]}],
  "education": [{"institution": "Universitat Freiburg", "area": "Physics", "studyType": "MSc", "startDate": "2012", "endDate": "2014"}],
  "languages": [{"language": "German", "fluency": "Native"}]
}`

func loadFixture(t *testing.T) *resume.Doc {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resume.json")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := resume.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func textBoxes(spec *Spec, page int) []TextBox {
	pg, ok := spec.Pages[itoa(page)]
	if !ok {
		return nil
	}
	return pg.Content.Text
}

func find(spec *Spec, page int, value string) (TextBox, bool) {
	for _, tb := range textBoxes(spec, page) {
		if tb.Value == value {
			return tb, true
		}
	}
	return TextBox{}, false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

func TestEntryHeadLocksTheDateColumn(t *testing.T) {
	doc := loadFixture(t)
	res := Compile(doc, Options{Lang: "en", Budget: 2, Density: &Densities[0]})

	head, ok := find(res.Spec, 1, "Bountystash")
	if !ok {
		t.Fatalf("no Bountystash head on page 1")
	}
	var meta *TextBox
	for _, tb := range textBoxes(res.Spec, 1) {
		if tb.Align == "right" && tb.Pos[1] == head.Pos[1] {
			box := tb
			meta = &box
		}
	}
	if meta == nil {
		t.Fatalf("no right-aligned meta on the head's baseline %v", head.Pos[1])
	}
	if meta.Pos[0] != rightEdge {
		t.Errorf("meta ends at %v, want %v", meta.Pos[0], rightEdge)
	}
	if meta.Value != "2023-01 – today" {
		t.Errorf("meta = %q", meta.Value)
	}
	if head.Font.Name != "Helvetica-Bold" {
		t.Errorf("head font = %q", head.Font.Name)
	}
}

func TestHeadersDrawTheSummaryAndContact(t *testing.T) {
	doc := loadFixture(t)
	res := Compile(doc, Options{Lang: "en", Budget: 2, Density: &Densities[0]})
	want := []string{"Matt Ji", "Senior Engineer"}
	for _, w := range want {
		if _, ok := find(res.Spec, 1, w); !ok {
			t.Errorf("missing %q on page 1", w)
		}
	}
	joined := ""
	for _, tb := range textBoxes(res.Spec, 1) {
		joined += tb.Value + "\n"
	}
	for _, w := range []string{"Risk Architect · Consultant", "matt@example.com · https://example.com · Freiburg · DE", "PROFILE", "EXPERIENCE"} {
		if !strings.Contains(joined, w) {
			t.Errorf("page 1 lacks %q\n%s", w, joined)
		}
	}
}

func TestGermanSectionTitles(t *testing.T) {
	doc := loadFixture(t)
	res := Compile(doc, Options{Lang: "de", Budget: 2, Density: &Densities[0]})
	joined := ""
	for _, tb := range textBoxes(res.Spec, 1) {
		joined += tb.Value + "\n"
	}
	for _, w := range []string{"PROFIL", "BERUFSERFAHRUNG", "KENNTNISSE", "AUSBILDUNG", "SPRACHEN"} {
		if !strings.Contains(joined, w) {
			t.Errorf("german page 1 lacks %q", w)
		}
	}
}

// A job head must never sit alone at the bottom of a page: its first bullet has
// to start on the same page, otherwise the next page opens mid-story.
func TestKeepTogetherKeepsTheFirstBullet(t *testing.T) {
	doc := loadFixture(t)
	filler := map[string]any{
		"name": "Filler", "position": "Engineer", "startDate": "2010", "endDate": "2011",
		"highlights": []any{"Filled a page with enough prose to force a break somewhere"},
	}
	work, _ := doc.Root["work"].([]any)
	for i := 0; i < 14; i++ {
		work = append(work, filler)
	}
	doc.Root["work"] = work

	for _, budget := range []int{2, 3} {
		res := Compile(doc, Options{Lang: "en", Budget: budget})
		if res.Pages < 2 {
			t.Fatalf("fixture did not overflow, pages = %d", res.Pages)
		}
		for page := 1; page <= res.Pages; page++ {
			for _, tb := range textBoxes(res.Spec, page) {
				if tb.Font.Name != "Helvetica-Bold" || tb.Value != "O3 Energy" {
					continue
				}
				bullet := "Modelled PPA exposure for a 40 MW solar portfolio"
				if _, ok := find(res.Spec, page, bullet); !ok {
					t.Errorf("budget %d: O3 Energy head on page %d without its bullet", budget, page)
				}
			}
		}
	}
}

func TestNothingRunsBelowTheBottomMargin(t *testing.T) {
	doc := loadFixture(t)
	res := Compile(doc, Options{Lang: "en", Budget: 2})
	for page := 1; page <= res.Pages; page++ {
		for _, tb := range textBoxes(res.Spec, page) {
			if tb.Pos[1] > PageHeight-26 {
				t.Errorf("page %d: %q at y=%v is below the footer", page, tb.Value, tb.Pos[1])
			}
		}
	}
}

func TestSolverTightensWhenTheCVOverflows(t *testing.T) {
	doc := loadFixture(t)
	work, _ := doc.Root["work"].([]any)
	for i := 0; i < 24; i++ {
		work = append(work, map[string]any{
			"name": "Company " + itoa(i), "position": "Engineer", "startDate": "2001", "endDate": "2002",
			"highlights": []any{
				"A bullet long enough to wrap onto a second line in the left column of the page",
				"Another bullet that also wraps once the column gets narrow enough to matter",
			},
		})
	}
	doc.Root["work"] = work

	res := Compile(doc, Options{Lang: "en", Budget: 2})
	if !res.Fits || res.Pages > 2 {
		t.Fatalf("solver did not fit: pages=%d fits=%v attempts=%+v", res.Pages, res.Fits, res.Attempts)
	}
	if len(res.Attempts) < 2 {
		t.Fatalf("expected a descent through the ladder, attempts=%+v", res.Attempts)
	}
	if res.Attempts[0].Pages <= 2 {
		t.Fatalf("the fixture fits at the first preset, so the solver was not exercised: %+v", res.Attempts)
	}
	if res.Density.ID == Densities[0].ID && res.Pages > 2 {
		t.Errorf("kept the widest preset on an overflowing CV")
	}
}

func TestGuidesAlsoTurnOnTheContentBox(t *testing.T) {
	doc := loadFixture(t)
	res := Compile(doc, Options{Lang: "en", Guides: true, Budget: 2})
	if !res.Spec.Guides || !res.Spec.ContentBox {
		t.Errorf("guides=%v contentBox=%v", res.Spec.Guides, res.Spec.ContentBox)
	}
	plain := Compile(doc, Options{Lang: "en", Budget: 2})
	if plain.Spec.Guides || plain.Spec.ContentBox {
		t.Error("guides leaked into a normal build")
	}
}

func TestSignatureIsPlacedAboveTheFooterAndReserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sig.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 120, 40))
	for x := 0; x < 120; x++ {
		img.Set(x, 10, color.Black)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	doc := loadFixture(t)
	res := Compile(doc, Options{
		Lang: "en", Budget: 2, SigSrc: "assets/signature.png", SigFile: path,
	})
	page := res.Spec.Pages[itoa(res.Pages)]
	if len(page.Content.Images) != 1 {
		t.Fatalf("images = %+v", page.Content.Images)
	}
	im := page.Content.Images[0]
	if im.Src != "assets/signature.png" {
		t.Errorf("src = %q", im.Src)
	}
	wantH := 92 * 40.0 / 120
	if im.Pos[1]+wantH > PageHeight-MarginBottom+0.5 {
		t.Errorf("signature bottom %v crosses the bottom margin", im.Pos[1]+wantH)
	}
	for _, tb := range page.Content.Text {
		if tb.Pos[1] > im.Pos[1]-1 && tb.Pos[1] < PageHeight-30 {
			t.Errorf("text %q at y=%v collides with the signature band", tb.Value, tb.Pos[1])
		}
	}
}

func TestFooterOnEveryPage(t *testing.T) {
	doc := loadFixture(t)
	res := Compile(doc, Options{Lang: "en", Budget: 2})
	for page := 1; page <= res.Pages; page++ {
		want := "page " + itoa(page) + " of " + itoa(res.Pages)
		if _, ok := find(res.Spec, page, want); !ok {
			t.Errorf("page %d lacks %q", page, want)
		}
	}
}

func TestCoverLetterSharesTheHeader(t *testing.T) {
	doc := loadFixture(t)
	letter := coverletter.Letter{
		Company:    "TestGmbH",
		Role:       "Platform Engineer",
		Salutation: "Dear Hiring Team,",
		Opening:    "I am applying for Platform Engineer at TestGmbH.",
		Body:       []string{"Built a retrieval layer over internal documents.", "Ran the evaluation harness."},
		Closing:    "Kind regards,",
		SourceLang: "en",
	}
	res := CompileLetter(doc, letter, Options{Lang: "en", Budget: 1})
	if res.Pages != 1 || !res.Fits {
		t.Fatalf("letter pages = %d", res.Pages)
	}
	joined := ""
	for _, tb := range textBoxes(res.Spec, 1) {
		joined += tb.Value + "\n"
	}
	for _, want := range []string{"Matt Ji", "TestGmbH", "Dear Hiring Team,", "Kind regards,", "Platform Engineer"} {
		if !strings.Contains(joined, want) {
			t.Errorf("letter lacks %q\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "PROFILE") {
		t.Error("the letter should not inherit CV sections")
	}
}
