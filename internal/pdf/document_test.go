package pdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaoyanji/jobby/internal/resume"
)

func docFor(t *testing.T, json string) *resume.Doc {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resume.json")
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := resume.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestBuildDocumentPassesDensityAndAttaches(t *testing.T) {
	h := newHarness(t)
	doc := docFor(t, `{"basics":{"name":"Ada","label":"Engineer"},"meta":{"language":"en"}}`)
	jsonPath := filepath.Join(h.root, "dist", "rendered.json")
	pdfPath := filepath.Join(h.root, "dist", "cv.pdf")
	capsulePath := filepath.Join(h.root, "capsule.json")
	if err := os.WriteFile(capsulePath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := BuildDocument(DocOptions{
		Root:        h.root,
		Template:    filepath.Join(h.root, "templates", "resume.typ"),
		JSONPath:    jsonPath,
		PDFPath:     pdfPath,
		Doc:         doc,
		Attachments: []string{capsulePath},
		AttachJSON:  true,
		Budget:      -1, // single compile, no solver
		Properties:  map[string]string{"Language": "en", "Lens": "ai"},
		Log:         func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("build: %v\ncalls:\n%s", err, h.calls(t))
	}
	if res.Preset.Label != "10pt / normal" {
		t.Errorf("preset = %q", res.Preset.Label)
	}
	calls := h.calls(t)
	for _, want := range []string{
		"--input font_size=10",
		"--input v_spacing=0.65",
		"attachments add -q " + pdfPath + " " + jsonPath + " " + capsulePath,
		"properties add -q " + pdfPath + " Language=en",
		"Lens=ai",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("missing %q\nlog:\n%s", want, calls)
		}
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("rendered JSON not written: %v", err)
	}
}

func TestBuildDocumentTightensUntilItFits(t *testing.T) {
	h := newHarness(t)
	doc := docFor(t, `{"basics":{"name":"Ada"},"meta":{"language":"en"}}`)
	// The stub reports 2 pages, so that is the best possible fit.
	res, err := BuildDocument(DocOptions{
		Root:     h.root,
		Template: filepath.Join(h.root, "templates", "resume.typ"),
		JSONPath: filepath.Join(h.root, "dist", "rendered.json"),
		PDFPath:  filepath.Join(h.root, "dist", "cv.pdf"),
		Doc:      doc,
		Budget:   1,
		Log:      func(string, ...any) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Fitted {
		t.Error("claimed a 1-page fit when the stub always says 2")
	}
	if len(res.Attempts) != len(Presets()) {
		t.Errorf("attempts = %d", len(res.Attempts))
	}
	if got := strings.Count(h.calls(t), "font_size="); got != len(Presets()) {
		t.Errorf("compiled %d times, want %d", got, len(Presets()))
	}
}

func TestBuildDocumentWritesPayload(t *testing.T) {
	h := newHarness(t)
	jsonPath := filepath.Join(h.root, "dist", "letter.json")
	payload := map[string]any{"letter": map[string]any{"opening": "Hello"}}
	if _, err := BuildDocument(DocOptions{
		Root:     h.root,
		Template: filepath.Join(h.root, "templates", "cover_letter.typ"),
		JSONPath: jsonPath,
		PDFPath:  filepath.Join(h.root, "dist", "letter.pdf"),
		Payload:  payload,
		Budget:   -1,
		Log:      func(string, ...any) {},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"opening": "Hello"`) {
		t.Errorf("payload = %s", raw)
	}
}

func TestMergeCopiesWhenThereIsOneDocument(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.pdf")
	out := filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(in, []byte("%PDF-1.4 one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Merge(out, []string{in}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "%PDF-1.4 one" {
		t.Fatalf("merge copy = %q, %v", b, err)
	}
	if err := Merge(out, nil); err == nil {
		t.Error("expected an error for an empty merge")
	}
}
