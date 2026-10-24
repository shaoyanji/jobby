// Package pdf turns an exported resume into a stamped, self-describing PDF.
//
// The chain is deliberately boring and observable: export JSON, compile with
// typst, post-process with pdfcpu, then ask pdfcpu what it produced. Every step
// reports the exact argv, so a failure is reproducible by hand.
package pdf

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shaoyanji/jobby/internal/export"
	"github.com/shaoyanji/jobby/internal/resume"
)

// Options is one build.
type Options struct {
	Root         string
	Lang         string
	Profile      string
	ResumePath   string
	CachePath    string
	LoadoutsPath string
	Template     string
	OutPDF       string
	OutJSON      string

	StampSig     bool
	AttachJSON   bool
	ReloadMupdf  bool
	Optimize     bool
	SkipOptimize bool
	PageBudget   int
	Log          func(format string, args ...any)
	// NoNetwork disables the first-run asset download, for offline builds.
	NoNetwork bool
}

// Result reports what landed on disk.
type Result struct {
	PDF      string
	JSON     string
	Pages    int
	Bytes    int64
	Warnings []string
}

func (o *Options) logf(format string, args ...any) {
	if o.Log != nil {
		o.Log(format, args...)
	}
}

func (o *Options) defaults() error {
	if o.Root == "" {
		o.Root = "."
	}
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return err
	}
	o.Root = root
	if o.Lang == "" {
		o.Lang = "en"
	}
	if o.ResumePath == "" {
		o.ResumePath = filepath.Join(root, "resume.json")
	}
	if o.CachePath == "" {
		o.CachePath = filepath.Join(root, "i18n_cache.json")
	}
	if o.LoadoutsPath == "" {
		o.LoadoutsPath = filepath.Join(root, "loadouts.json")
	}
	if o.Template == "" {
		o.Template = filepath.Join(root, "templates", "resume.typ")
	}
	dist := filepath.Join(root, "dist")
	if o.OutJSON == "" {
		o.OutJSON = filepath.Join(dist, "resume-"+o.Lang+".json")
	}
	if o.OutPDF == "" {
		o.OutPDF = filepath.Join(dist, "resume-"+o.Lang+".pdf")
	}
	if o.PageBudget == 0 {
		o.PageBudget = 2
	}
	return nil
}

// Build runs the whole pipeline and returns the artifact facts.
func Build(opt Options) (*Result, error) {
	if err := opt.defaults(); err != nil {
		return nil, err
	}
	if !opt.SkipOptimize {
		opt.Optimize = true
	}
	res := &Result{PDF: opt.OutPDF, JSON: opt.OutJSON}

	exp, err := export.Run(export.Request{
		ResumePath:   opt.ResumePath,
		CachePath:    opt.CachePath,
		LoadoutsPath: opt.LoadoutsPath,
		Lang:         opt.Lang,
		Profile:      opt.Profile,
		SaveLoadouts: true,
	})
	if err != nil {
		return nil, err
	}
	assets, warnings := ResolveAssets(exp.Doc, opt.Root, !opt.NoNetwork)
	res.Warnings = append(res.Warnings, warnings...)
	for kind, a := range assets {
		opt.logf("asset   %s (%s)", a, kind)
	}

	if _, err := exp.Write(opt.OutJSON, nil); err != nil {
		return nil, err
	}
	opt.logf("export  %s (%d translated, %d missing)", rel(opt.Root, opt.OutJSON), exp.Translated, exp.Missing)

	if _, err := exec.LookPath("typst"); err != nil {
		return nil, errors.New("typst not found on PATH (run this inside `nix develop`)")
	}
	// typst resolves --input paths against --root, so an absolute path would
	// be read as root-relative (the leading slash is stripped).
	typstArgs := []string{
		"compile",
		"--root", opt.Root,
		"--input", "json_path=" + typstPath(opt.Root, opt.OutJSON),
	}
	if avatar := assets["avatar"]; avatar != "" {
		typstArgs = append(typstArgs, "--input", "avatar="+typstPath(opt.Root, avatar))
	}
	typstArgs = append(typstArgs, opt.Template, opt.OutPDF)
	if err := run(opt, "typst", typstArgs...); err != nil {
		return nil, err
	}
	opt.logf("typst   %s", rel(opt.Root, opt.OutPDF))

	if opt.StampSig {
		sig := findSignature(opt.Root)
		switch {
		case sig == "":
			opt.logf("stamp   skipped (no assets/signature.{png,jpg})")
		default:
			if err := stampSignature(opt.Root, sig, opt.OutPDF); err != nil {
				return nil, err
			}
			opt.logf("stamp   %s", rel(opt.Root, sig))
		}
	}

	if opt.AttachJSON {
		if err := run(opt, "pdfcpu", "attachments", "add", "-q", opt.OutPDF, opt.OutJSON); err != nil {
			// An already-attached file is not a build failure.
			if strings.Contains(err.Error(), "already") {
				opt.logf("attach  skipped (already attached)")
			} else {
				return nil, err
			}
		} else {
			opt.logf("attach  %s", rel(opt.Root, opt.OutJSON))
		}
	}

	props := []string{
		"Language=" + opt.Lang,
		"Profile=" + nonEmpty(opt.Profile, "default"),
		"BuildTimestamp=" + time.Now().UTC().Format(time.RFC3339),
	}
	if task := exp.Doc.MetaString("last_synced"); task != "" {
		props = append(props, "SyncedTask="+task)
	}
	if len(exp.Doc.MetaStrings("synced_tasks")) > 0 {
		tasks := exp.Doc.MetaStrings("synced_tasks")
		props = append(props, "SyncedTasks="+strings.Join(tasks, ","))
	}
	propArgs := append([]string{"properties", "add", "-q", opt.OutPDF}, props...)
	if err := run(opt, "pdfcpu", propArgs...); err != nil {
		res.Warnings = append(res.Warnings, "properties: "+err.Error())
		opt.logf("props   failed: %v", err)
	} else {
		opt.logf("props   %s", strings.Join(props, " "))
	}

	if opt.Optimize {
		tmp := opt.OutPDF + ".opt.pdf"
		if err := run(opt, "pdfcpu", "optimize", "-q", opt.OutPDF, tmp); err != nil {
			res.Warnings = append(res.Warnings, "optimize: "+err.Error())
			opt.logf("optim   failed: %v", err)
			_ = os.Remove(tmp)
		} else {
			if err := os.Rename(tmp, opt.OutPDF); err != nil {
				return nil, err
			}
			opt.logf("optim   done")
		}
	}

	info, err := Info(opt.OutPDF)
	if err != nil {
		res.Warnings = append(res.Warnings, "info: "+err.Error())
	} else {
		res.Pages = info.Pages
		if info.Pages > opt.PageBudget {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"%d pages exceeds the %d-page budget", info.Pages, opt.PageBudget))
		}
	}
	if st, err := os.Stat(opt.OutPDF); err == nil {
		res.Bytes = st.Size()
	}
	opt.logf("pdf     %s · %d pages · %.1f KiB", rel(opt.Root, opt.OutPDF), res.Pages, float64(res.Bytes)/1024)

	if opt.ReloadMupdf {
		Reload(opt.logf)
	}
	return res, nil
}

// PDFInfo is the subset of `pdfcpu info` worth reporting.
type PDFInfo struct {
	Pages  int
	Output string
}

var pageCountRe = regexp.MustCompile(`(?m)Page count:\s*(\d+)`)

// Info asks pdfcpu for the page count and full report.
func Info(path string) (PDFInfo, error) {
	cmd := exec.Command("pdfcpu", "info", path)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return PDFInfo{Output: text}, fmt.Errorf("pdfcpu info: %s", text)
	}
	info := PDFInfo{Output: text}
	if m := pageCountRe.FindStringSubmatch(text); len(m) == 2 {
		info.Pages, _ = strconv.Atoi(m[1])
	}
	return info, nil
}

// Mappe merges the German CV with an optional cover letter and reference
// bundle into one application packet.
func Mappe(opt Options) (string, error) {
	if err := opt.defaults(); err != nil {
		return "", err
	}
	opt.Lang = "de"
	opt.OutPDF = filepath.Join(opt.Root, "dist", "resume-de.pdf")
	built, err := Build(opt)
	if err != nil {
		return "", err
	}
	out := filepath.Join(opt.Root, "dist", "bewerbungsmappe-de.pdf")

	var parts []string
	if p := existing(filepath.Join(opt.Root, "assets", "anschreiben.pdf")); p != "" {
		parts = append(parts, p)
	}
	parts = append(parts, built.PDF)
	if p := existing(filepath.Join(opt.Root, "assets", "zeugnisse.pdf")); p != "" {
		parts = append(parts, p)
	}

	if len(parts) == 1 {
		b, err := os.ReadFile(built.PDF)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(out, b, 0o644); err != nil {
			return "", err
		}
		opt.logf("mappe   %s (single document, copied)", rel(opt.Root, out))
		return out, nil
	}

	args := append([]string{"merge", "-q", out}, parts...)
	out2, err := exec.Command("pdfcpu", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdfcpu merge: %s", strings.TrimSpace(string(out2)))
	}
	opt.logf("mappe   %s (%d documents)", rel(opt.Root, out), len(parts))
	return out, nil
}

// Unpack extracts the embedded JSON snapshot from a built PDF. It returns the
// extracted files.
func Unpack(pdfPath, dir string, log func(string, ...any)) ([]string, error) {
	if dir == "" {
		tmp, err := os.MkdirTemp("", "jobby-unpack-*")
		if err != nil {
			return nil, err
		}
		dir = tmp
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	before, _ := os.ReadDir(dir)
	seen := map[string]bool{}
	for _, e := range before {
		seen[e.Name()] = true
	}
	out, err := exec.Command("pdfcpu", "attachments", "extract", "-q", pdfPath, dir).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pdfcpu attachments extract: %s", strings.TrimSpace(string(out)))
	}
	after, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range after {
		if seen[e.Name()] {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	if log != nil {
		log("unpack  %d attachment(s) -> %s", len(files), dir)
	}
	return files, nil
}

// Reload signals a running mupdf so it re-reads the file. mupdf keeps its
// viewport across SIGHUP, which is the whole point of the loop.
func Reload(log func(string, ...any)) {
	if _, err := exec.LookPath("pkill"); err != nil {
		return
	}
	if err := exec.Command("pkill", "-HUP", "mupdf").Run(); err != nil {
		if log != nil {
			log("mupdf   no running viewer")
		}
		return
	}
	if log != nil {
		log("mupdf   reloaded")
	}
}

// Preview opens a PDF in mupdf without blocking.
func Preview(path string) error {
	cmd := exec.Command("mupdf", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mupdf: %w", err)
	}
	return cmd.Process.Release()
}

// ResolveAssets resolves the local file for each remote signature/avatar once,
// so typst and pdfcpu read a local image. The document is left untouched: the
// exported JSON is attached to the PDF and mailed on, so a build-machine path
// has no business in it.
func ResolveAssets(doc *resume.Doc, root string, network bool) (map[string]string, []string) {
	resolved := map[string]string{}
	var warnings []string
	assetsDir := filepath.Join(root, "assets")
	for _, spec := range []struct{ key, base string }{
		{"basics.signature", "signature"},
		{"basics.image", "avatar"},
	} {
		value, ok := doc.GetString(spec.key)
		if !ok || value == "" {
			continue
		}
		if !strings.HasPrefix(value, "http") {
			if st, err := os.Stat(value); err == nil && st.Size() > 0 {
				resolved[spec.base] = value
			}
			continue
		}
		if existing := findAsset(assetsDir, spec.base); existing != "" {
			resolved[spec.base] = existing
			continue
		}
		if !network {
			continue
		}
		dest := filepath.Join(assetsDir, spec.base+extFromURL(value))
		if err := os.MkdirAll(assetsDir, 0o755); err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if err := download(value, dest); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", spec.key, err))
			continue
		}
		resolved[spec.base] = dest
	}
	return resolved, warnings
}

func existing(path string) string {
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return path
	}
	return ""
}

func findAsset(dir, base string) string {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".svg"} {
		p := filepath.Join(dir, base+ext)
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p
		}
	}
	return ""
}

// findSignature only accepts raster formats: pdfcpu's image stamp mode cannot
// place an SVG.
func findSignature(root string) string {
	for _, ext := range []string{".png", ".jpg", ".jpeg"} {
		p := filepath.Join(root, "assets", "signature"+ext)
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p
		}
	}
	return ""
}

func extFromURL(u string) string {
	clean := u
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	switch strings.ToLower(filepath.Ext(clean)) {
	case ".png":
		return ".png"
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".svg":
		return ".svg"
	default:
		return ".png"
	}
}

func download(url, dest string) error {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 8<<20)); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func run(opt Options, name string, args ...string) error {
	return runIn(opt.Root, name, args...)
}

func runIn(root, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

// stampSignature places the signature image bottom-right on the last page,
// writing through a temp file so a failed stamp cannot corrupt the PDF.
func stampSignature(root, sig, pdfPath string) error {
	tmp := pdfPath + ".stamp.pdf"
	// Short flags on purpose: pflag reads "-mode image" as "-m ode".
	err := runIn(root, "pdfcpu", "stamp", "add", "-m", "image", "-p", "l", "--",
		sig, "pos:br, off:-40 35, scale:0.18 abs, rot:0", pdfPath, tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, pdfPath)
}

// typstPath renders a path the way typst wants it: root-anchored ("/dist/x")
// because the template resolves json() relative to its own directory.
func typstPath(root, path string) string {
	r := rootRelative(root, path)
	if strings.HasPrefix(r, "..") {
		return r
	}
	return "/" + r
}

func rootRelative(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(r, "..") {
		return path
	}
	return r
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}

func nonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
