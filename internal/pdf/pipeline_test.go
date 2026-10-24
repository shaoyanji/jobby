package pdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const resumeJSON = `{
  "basics": {"name": "Ada", "label": "Platform Engineer", "summary": "Builds systems"},
  "work": [{"name": "O3", "position": "Engineer", "highlights": ["One"]}],
  "projects": [{"name": "resumectl", "highlights": ["Lossless"]}],
  "meta": {"language": "en", "last_synced": "20261024-default", "synced_tasks": ["20261024-default"]}
}`

type harness struct {
	root string
	log  string
	bin  string
}

func newHarness(t *testing.T) harness {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls.log")
	if err := os.WriteFile(filepath.Join(root, "resume.json"), []byte(resumeJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "templates", "resume.typ"), []byte("// stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStub(t, bin, "typst", `#!/bin/sh
echo "typst $*" >> "$STUB_LOG"
last=""
for a in "$@"; do last="$a"; done
printf '%%PDF-1.4 stub\n' > "$last"
`)
	writeStub(t, bin, "pdfcpu", `#!/bin/sh
echo "pdfcpu $*" >> "$STUB_LOG"
case "$1" in
  info) echo "Page count:           2" ;;
  stamp)
    last=""; prev=""
    for a in "$@"; do prev="$last"; last="$a"; done
    cp "$prev" "$last" ;;
  optimize)
    last=""
    for a in "$@"; do last="$a"; done
    cp "$3" "$last" ;;
  attachments)
    if [ "$2" = "extract" ]; then
      last=""
      for a in "$@"; do last="$a"; done
      printf '{"extracted":true}\n' > "$last/active-de.json"
    fi ;;
esac
`)
	t.Setenv("STUB_LOG", log)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return harness{root: root, log: log, bin: bin}
}

func writeStub(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (h harness) withSignature(t *testing.T) harness {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(h.root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, "assets", "signature.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h harness) calls(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(h.log)
	if err != nil {
		return ""
	}
	return string(b)
}

func TestBuildRunsTheWholeChain(t *testing.T) {
	h := newHarness(t).withSignature(t)
	var lines []string
	res, err := Build(Options{
		Root:       h.root,
		Lang:       "de",
		Profile:    "default",
		StampSig:   true,
		AttachJSON: true,
		Optimize:   true,
		NoNetwork:  true,
		PageBudget: 1,
		Log:        func(f string, a ...any) { lines = append(lines, f) },
	})
	if err != nil {
		t.Fatalf("build: %v\ncalls:\n%s", err, h.calls(t))
	}
	if res.Pages != 2 {
		t.Errorf("pages = %d, want 2 from the stub", res.Pages)
	}
	if res.Bytes == 0 {
		t.Error("pdf was not written")
	}
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "page budget") {
		t.Errorf("warnings = %v, want a page-budget warning", res.Warnings)
	}

	calls := h.calls(t)
	for _, want := range []string{
		"typst compile",
		"--input json_path=/dist/resume-de.json",
		"pdfcpu stamp add -m image -p l",
		"pdfcpu attachments add -q",
		"pdfcpu properties add -q",
		"properties add -q " + res.PDF + " Language=de",
		"Profile=default",
		"SyncedTask=20261024-default",
		"pdfcpu optimize -q",
		"pdfcpu info",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("missing call %q\nfull log:\n%s", want, calls)
		}
	}

	raw, err := os.ReadFile(res.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"language": "de"`) {
		t.Errorf("exported JSON is not stamped de:\n%s", raw)
	}
	if !strings.Contains(string(raw), "20261024-default") {
		t.Error("synced_tasks was dropped from the export")
	}
}

func TestBuildSkipsStampWithoutSignature(t *testing.T) {
	h := newHarness(t)
	if _, err := Build(Options{Root: h.root, Lang: "en", StampSig: true, NoNetwork: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h.calls(t), "pdfcpu stamp") {
		t.Error("stamped without assets/signature.*")
	}
}

func TestBuildRefusesWithoutTypst(t *testing.T) {
	h := newHarness(t)
	// Keep pdfcpu available but hide typst.
	empty := t.TempDir()
	if err := os.Symlink(filepath.Join(h.bin, "pdfcpu"), filepath.Join(empty, "pdfcpu")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", empty)
	_, err := Build(Options{Root: h.root, Lang: "en", NoNetwork: true})
	if err == nil || !strings.Contains(err.Error(), "typst not found") {
		t.Fatalf("err = %v, want a missing-typst error", err)
	}
}

func TestBuildReportsATypstFailure(t *testing.T) {
	h := newHarness(t)
	writeStub(t, h.bin, "typst", `#!/bin/sh
echo "error: unknown field" >&2
exit 3
`)
	_, err := Build(Options{Root: h.root, Lang: "en", NoNetwork: true})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("err = %v, want typst stderr in the error", err)
	}
}

func TestInfoParsesPageCount(t *testing.T) {
	newHarness(t)
	info, err := Info("fake.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if info.Pages != 2 {
		t.Errorf("pages = %d", info.Pages)
	}
}

func TestUnpackExtractsAttachments(t *testing.T) {
	newHarness(t)
	dir := t.TempDir()
	files, err := Unpack("fake.pdf", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !strings.HasSuffix(files[0], "active-de.json") {
		t.Fatalf("files = %v", files)
	}
	if _, err := os.Stat(files[0]); err != nil {
		t.Errorf("extracted file missing: %v", err)
	}
}

func TestMappeCopiesWhenNothingToMerge(t *testing.T) {
	h := newHarness(t)
	out, err := Mappe(Options{Root: h.root, NoNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(h.root, "dist", "bewerbungsmappe-de.pdf")
	if out != want {
		t.Errorf("out = %q, want %q", out, want)
	}
	b, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "%PDF") {
		t.Errorf("mappe is not a PDF: %q", b)
	}
}
