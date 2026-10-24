package pdf

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/shaoyanji/jobby/internal/layout"
)

// Engine shells out to pdfcpu. Every PDF jobby writes goes through it, so there
// is exactly one renderer to reason about: `pdfcpu create` for new pages,
// `pdfcpu merge` for the package and pdfcpu's attachment/property commands for
// provenance.
type Engine struct {
	// Root is the working directory: pdfcpu resolves relative image paths
	// against it, which is why the compiler emits "assets/signature.png".
	Root string
	// Bin is the pdfcpu executable; empty means the one on PATH.
	Bin string
}

// NewEngine returns an engine rooted at root.
func NewEngine(root string) *Engine { return &Engine{Root: root} }

func (e *Engine) bin() string {
	if e.Bin != "" {
		return e.Bin
	}
	return "pdfcpu"
}

func (e *Engine) run(args ...string) (string, error) {
	cmd := exec.Command(e.bin(), args...)
	cmd.Dir = e.Root
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("pdfcpu %s: %w", strings.Join(args, " "), err)
		}
		return text, fmt.Errorf("pdfcpu %s: %s", strings.Join(args[0:1], " "), firstLine(text))
	}
	return text, nil
}

// Check verifies pdfcpu is present and new enough for `create` with a layout
// spec. A missing or old binary is a setup problem, so the message says how to
// fix it rather than leaking the command line.
func (e *Engine) Check() (string, error) {
	out, err := e.run("version")
	if err != nil {
		return "", fmt.Errorf("pdfcpu not found (run inside `nix develop`): %w", err)
	}
	v := strings.TrimPrefix(versionRe.FindString(strings.TrimSpace(out)), "v")
	if v == "" {
		return "", fmt.Errorf("cannot read pdfcpu version from %q", firstLine(out))
	}
	parts := strings.Split(v, ".")
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if major == 0 && minor < 16 {
		return v, fmt.Errorf("pdfcpu %s is too old: the layout compiler needs >= 0.16 (create with guides/contentBox)", v)
	}
	return v, nil
}

var versionRe = regexp.MustCompile(`v?(\d+\.\d+(?:\.\d+)?)`)

// Create compiles spec into pdfPath. The layout JSON is kept next to the PDF so
// an alignment question can be answered by reading the actual coordinates.
func (e *Engine) Create(spec *layout.Spec, jsonPath, pdfPath string) error {
	if err := spec.WriteJSON(jsonPath); err != nil {
		return err
	}
	// pdfcpu appends when the output already exists; a rebuild must replace it
	// rather than stack a second copy behind the first.
	if err := os.Remove(pdfPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := e.run("create", "-q", jsonPath, pdfPath); err != nil {
		return err
	}
	return e.Validate(pdfPath)
}

// Validate runs pdfcpu's structural validation.
func (e *Engine) Validate(pdfPath string) error {
	_, err := e.run("validate", "-q", pdfPath)
	return err
}

var infoPageCountRe = regexp.MustCompile(`Page count:\s*(\d+)`)

// Pages reads the page count back from the rendered file. The compiler already
// knows it, but asking the PDF proves the layout survived pdfcpu.
func (e *Engine) Pages(pdfPath string) (int, error) {
	out, err := e.run("info", pdfPath)
	if err != nil {
		return 0, err
	}
	m := infoPageCountRe.FindStringSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("no page count in pdfcpu info for %s", filepath.Base(pdfPath))
	}
	return strconv.Atoi(m[1])
}

// Attach embeds the given files in the PDF. This is the whole version-control
// story: the capsule, the rendered resume and the layout coordinates travel
// inside the document.
func (e *Engine) Attach(pdfPath string, files ...string) error {
	if len(files) == 0 {
		return nil
	}
	// pdfcpu names an attachment after its base name, so a second `rendered.json`
	// from another application would collide. The callers pass unique names.
	args := append([]string{"attachments", "add", "-q", pdfPath}, files...)
	_, err := e.run(args...)
	return err
}

// Props writes document metadata; pdfcpu wants "key = value" strings.
func (e *Engine) Props(pdfPath string, pairs []string) error {
	if len(pairs) == 0 {
		return nil
	}
	args := append([]string{"properties", "add", "-q", pdfPath}, pairs...)
	_, err := e.run(args...)
	return err
}

// Optimize rewrites the file with pdfcpu's optimizer.
func (e *Engine) Optimize(pdfPath string) error {
	_, err := e.run("optimize", "-q", pdfPath)
	return err
}

// Merge concatenates documents in order.
func (e *Engine) Merge(out string, parts []string) error {
	if len(parts) == 0 {
		return errors.New("merge needs at least one document")
	}
	// pdfcpu refuses to overwrite, so a rebuild removes yesterday's package.
	if err := os.Remove(out); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	args := append([]string{"merge", "-q", out}, parts...)
	_, err := e.run(args...)
	return err
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
