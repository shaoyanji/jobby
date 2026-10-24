package pdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaoyanji/jobby/internal/layout"
)

// stubPDFCPU writes a shell stub that refuses to overwrite, like the real
// binary does, so the engine's "remove first" behaviour is what is under test.
func stubPDFCPU(t *testing.T, dir, version string) string {
	t.Helper()
	body := `#!/bin/sh
case "$1" in
  version) echo "version: ` + version + `" ;;
  create)
    last=""
    for a in "$@"; do last="$a"; done
    [ -e "$last" ] && { echo "refusing to overwrite $last" >&2; exit 1; }
    printf '%%PDF-1.4 stub\n' > "$last" ;;
  merge)
    [ -e "$3" ] && { echo "refusing to overwrite $3" >&2; exit 1; }
    cp "$4" "$3" ;;
  validate|attachments|properties|optimize) : ;;
esac
`
	path := filepath.Join(dir, "pdfcpu")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCreateReplacesThePreviousPDF(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{Root: dir, Bin: stubPDFCPU(t, dir, "v0.16.1")}
	out := filepath.Join(dir, "cv.pdf")
	if err := os.WriteFile(out, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := &layout.Spec{Paper: "A4", Origin: "UpperLeft", Pages: map[string]layout.Page{}}
	if err := e.Create(spec, filepath.Join(dir, "layout.json"), out); err != nil {
		t.Fatalf("create over an existing file: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "%PDF") {
		t.Errorf("output = %q", b)
	}
}

func TestMergeReplacesThePreviousPackage(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{Root: dir, Bin: stubPDFCPU(t, dir, "v0.16.1")}
	out := filepath.Join(dir, "package.pdf")
	part := filepath.Join(dir, "part.pdf")
	if err := os.WriteFile(part, []byte("%PDF-1.4 part"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.Merge(out, []string{part, part}); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	if err := e.Merge(out, []string{part, part}); err != nil {
		t.Fatalf("rebuild over the existing package: %v", err)
	}
}

func TestCheckRejectsAPDFCPUWithoutCreate(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{Root: dir, Bin: stubPDFCPU(t, dir, "v0.11.0")}
	_, err := e.Check()
	if err == nil || !strings.Contains(err.Error(), ">= 0.16") {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckAcceptsThePinnedVersion(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{Root: dir, Bin: stubPDFCPU(t, dir, "v0.16.1")}
	v, err := e.Check()
	if err != nil {
		t.Fatal(err)
	}
	if v != "0.16.1" {
		t.Errorf("version = %q", v)
	}
}
