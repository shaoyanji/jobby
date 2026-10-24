package pdf

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shaoyanji/jobby/internal/atomicfile"
	"github.com/shaoyanji/jobby/internal/resume"
)

// DocOptions is one typst document build with an optional auto-fit loop.
type DocOptions struct {
	Root     string
	Template string
	// JSONPath is written from Doc or Payload, read by typst and attached.
	JSONPath string
	PDFPath  string
	// Doc is the rendered resume; Payload is any other JSON (a cover letter).
	Doc     *resume.Doc
	Payload any
	// Inputs are extra typst --input values.
	Inputs map[string]string
	// Attachments are extra files embedded in the PDF.
	Attachments []string
	AttachJSON  bool
	StampSig    bool
	Optimize    bool
	// Budget is the page target. 0 means 2; a negative value compiles once with
	// the comfortable preset and does not solve.
	Budget     int
	Properties map[string]string
	Log        func(format string, args ...any)
}

// DocResult reports what was produced and how it was fitted.
type DocResult struct {
	PDF      string
	Pages    int
	Preset   Preset
	Fitted   bool
	Attempts []Attempt
	Bytes    int64
}

func (o *DocOptions) logf(format string, args ...any) {
	if o.Log != nil {
		o.Log(format, args...)
	}
}

// BuildDocument renders a typst template, optionally tightening the layout
// until the page count fits, then stamps, attaches, tags and optimizes it.
func BuildDocument(o DocOptions) (*DocResult, error) {
	if o.Root == "" {
		o.Root = "."
	}
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return nil, err
	}
	o.Root = root
	if o.Template == "" {
		o.Template = filepath.Join(root, "templates", "resume.typ")
	}
	if o.JSONPath == "" || o.PDFPath == "" {
		return nil, fmt.Errorf("document build needs JSONPath and PDFPath")
	}
	if o.Doc != nil {
		if err := atomicfile.WriteJSON(o.JSONPath, o.Doc.Root); err != nil {
			return nil, err
		}
	} else if o.Payload != nil {
		if err := atomicfile.WriteJSON(o.JSONPath, o.Payload); err != nil {
			return nil, err
		}
	}
	if _, err := exec.LookPath("typst"); err != nil {
		return nil, fmt.Errorf("typst not found on PATH (run this inside `nix develop`)")
	}

	compile := func(preset Preset) (string, error) {
		args := []string{
			"compile",
			"--root", o.Root,
			"--input", "json_path=" + typstPath(o.Root, o.JSONPath),
			"--input", "font_size=" + strconv.FormatFloat(preset.FontSize, 'f', -1, 64),
			"--input", "v_spacing=" + strconv.FormatFloat(preset.VSpacing, 'f', -1, 64),
		}
		for k, v := range o.Inputs {
			args = append(args, "--input", k+"="+v)
		}
		args = append(args, o.Template, o.PDFPath)
		if err := runIn(o.Root, "typst", args...); err != nil {
			return "", err
		}
		return o.PDFPath, nil
	}

	res := &DocResult{PDF: o.PDFPath}
	if o.Budget < 0 {
		preset := presets[0]
		if _, err := compile(preset); err != nil {
			return nil, err
		}
		res.Preset = preset
		pages, err := pageCount(o.PDFPath)
		if err != nil {
			return nil, err
		}
		res.Pages, res.Fitted = pages, true
	} else {
		fit, err := Fit(o.Budget, compile, func(pdf string) (int, error) { return pageCount(pdf) })
		res.Preset, res.Pages, res.Fitted, res.Attempts = fit.Preset, fit.Pages, fit.Fitted, fit.Attempts
		if err != nil {
			return nil, err
		}
		if !fit.Fitted {
			o.logf("autofit warning: still %d pages at the densest preset (%s)", fit.Pages, fit.Preset.Label)
		} else if len(fit.Attempts) > 1 {
			o.logf("autofit %s · %d pages (after %d attempts)", fit.Preset.Label, fit.Pages, len(fit.Attempts))
		} else {
			o.logf("autofit %s · %d pages", fit.Preset.Label, fit.Pages)
		}
	}

	if o.StampSig {
		if sig := findSignature(o.Root); sig == "" {
			o.logf("stamp   skipped (no assets/signature.{png,jpg})")
		} else if err := stampSignature(o.Root, sig, o.PDFPath); err != nil {
			return nil, err
		} else {
			o.logf("stamp   %s", rel(o.Root, sig))
		}
	}

	attach := make([]string, 0, len(o.Attachments)+1)
	if o.AttachJSON {
		attach = append(attach, o.JSONPath)
	}
	attach = append(attach, o.Attachments...)
	if len(attach) > 0 {
		args := append([]string{"attachments", "add", "-q", o.PDFPath}, attach...)
		if err := runIn(o.Root, "pdfcpu", args...); err != nil {
			if !strings.Contains(err.Error(), "already") {
				return nil, err
			}
			o.logf("attach  skipped (already attached)")
		} else {
			for _, a := range attach {
				o.logf("attach  %s", filepath.Base(a))
			}
		}
	}

	props := make([]string, 0, len(o.Properties))
	for k, v := range o.Properties {
		if v == "" {
			continue
		}
		props = append(props, k+"="+v)
	}
	// Sorted so two builds of the same state produce the same leaf values.
	sort.Strings(props)
	if len(props) > 0 {
		args := append([]string{"properties", "add", "-q", o.PDFPath}, props...)
		if err := runIn(o.Root, "pdfcpu", args...); err != nil {
			return nil, err
		}
	}

	if o.Optimize {
		tmp := o.PDFPath + ".opt.pdf"
		if err := runIn(o.Root, "pdfcpu", "optimize", "-q", o.PDFPath, tmp); err != nil {
			o.logf("optim   failed: %v", err)
			_ = os.Remove(tmp)
		} else if err := os.Rename(tmp, o.PDFPath); err != nil {
			return nil, err
		} else {
			o.logf("optim   done")
		}
	}

	if st, err := os.Stat(o.PDFPath); err == nil {
		res.Bytes = st.Size()
	}
	o.logf("pdf     %s · %d pages · %.1f KiB", rel(o.Root, o.PDFPath), res.Pages, float64(res.Bytes)/1024)
	return res, nil
}

// Merge joins several PDFs into one file.
func Merge(out string, parts []string) error {
	switch len(parts) {
	case 0:
		return fmt.Errorf("nothing to merge")
	case 1:
		b, err := os.ReadFile(parts[0])
		if err != nil {
			return err
		}
		return atomicfile.Write(out, b, 0o644)
	}
	args := append([]string{"merge", "-q", out}, parts...)
	cmd := exec.Command("pdfcpu", args...)
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pdfcpu merge: %s", strings.TrimSpace(string(combined)))
	}
	return nil
}

func pageCount(pdf string) (int, error) {
	info, err := Info(pdf)
	if err != nil {
		return 0, err
	}
	if info.Pages == 0 {
		return 0, fmt.Errorf("pdfcpu reported no page count for %s", pdf)
	}
	return info.Pages, nil
}
