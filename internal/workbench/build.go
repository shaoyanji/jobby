// Package workbench ties the pieces together: a capsule is turned into a
// fitted CV, a matching cover letter, and one merged application package, with
// every decision embedded in the PDFs.
package workbench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shaoyanji/jobby/internal/atomicfile"
	"github.com/shaoyanji/jobby/internal/capsule"
	"github.com/shaoyanji/jobby/internal/coverletter"
	"github.com/shaoyanji/jobby/internal/i18n"
	"github.com/shaoyanji/jobby/internal/layout"
	"github.com/shaoyanji/jobby/internal/lens"
	"github.com/shaoyanji/jobby/internal/pdf"
	"github.com/shaoyanji/jobby/internal/resume"
)

// Options is one capsule build.
type Options struct {
	Root         string
	ResumePath   string
	CachePath    string
	LensesPath   string
	GlossaryPath string

	Capsule *capsule.Capsule
	// LensID overrides the capsule's lens (used by `build --lens`).
	LensID string
	Lang   string
	// Points are cover-letter talking points. Empty means "use the lens
	// highlights", which is what makes a headless build produce real prose.
	Points []string

	Budget int
	// Guides draws pdfcpu's alignment grid and the content box into the PDF.
	Guides bool
	// Density forces one typography preset instead of solving for the budget.
	Density *layout.Density
	// NoSignature leaves the signature image out of the layout.
	NoSignature bool
	Optimize    bool
	NoCover     bool
	NoNetwork   bool
	SkipSync    bool
	ReloadMupdf bool
	Translator  i18n.Translator
	// Delay between translation calls. Zero means the 250 ms default.
	Delay time.Duration
	Log   func(format string, args ...any)
}

// Report describes a finished build.
type Report struct {
	CapsuleDir   string
	Lang         string
	CV           string
	Letter       string
	Package      string
	Layout       string
	LayoutLetter string
	Dist         []string
	Pages        int
	Preset       string
	Fitted       bool
	Translated   int
	Missing      int
	Stats        i18n.Stats
	Warnings     []string
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
	if o.ResumePath == "" {
		o.ResumePath = filepath.Join(root, "resume.json")
	}
	if o.CachePath == "" {
		o.CachePath = filepath.Join(root, "i18n_cache.json")
	}
	if o.LensesPath == "" {
		o.LensesPath = filepath.Join(root, "lenses.json")
	}
	if o.GlossaryPath == "" {
		o.GlossaryPath = filepath.Join(root, "glossary.json")
	}
	if o.Translator == nil {
		o.Translator = i18n.NewTransCLI()
	}
	if o.Budget == 0 {
		o.Budget = 2
	}
	return nil
}

// Build renders one application capsule.
func Build(o Options) (*Report, error) {
	if err := o.defaults(); err != nil {
		return nil, err
	}
	if o.Capsule == nil {
		return nil, fmt.Errorf("build needs a capsule")
	}
	doc, err := resume.Load(o.ResumePath)
	if err != nil {
		return nil, err
	}
	lenses, err := lens.Load(o.LensesPath)
	if err != nil {
		return nil, err
	}
	cache, err := i18n.Load(o.CachePath)
	if err != nil {
		return nil, err
	}
	glossary, err := i18n.LoadGlossary(o.GlossaryPath)
	if err != nil {
		return nil, err
	}

	c := o.Capsule
	previousLens := c.Lens
	if o.LensID != "" {
		c.Lens = o.LensID
	}
	lensChanged := previousLens != "" && previousLens != c.Lens
	l, ok := lenses.Get(c.Lens)
	if !ok {
		return nil, fmt.Errorf("unknown lens %q (have: %s)", c.Lens, strings.Join(lenses.IDs(), ", "))
	}
	c.Lens = l.ID
	if o.Lang != "" {
		c.Language = o.Lang
	}
	if c.Language == "" {
		c.Language = "en"
	}
	if strings.TrimSpace(c.Role) == "" {
		c.Role = l.Label
	}
	if c.FileStem == "" {
		c.FileStem = doc.MetaString("file_stem")
	}
	if c.FileStem == "" {
		c.FileStem = capsule.DefaultFileStem(doc.Name())
	}
	if c.Label == "" {
		c.Label = l.Label
	}
	if lensChanged {
		// An explicit switch to another lens re-seeds the story: keeping the old
		// lens's bullets under the new persona is how a CV starts lying.
		c.Label = l.Label
		c.Titles = copyStringMap(l.Titles)
		c.Work, c.Projects = l.Selection(doc)
	} else {
		if len(c.Titles) == 0 && len(l.Titles) > 0 {
			c.Titles = copyStringMap(l.Titles)
		}
		if len(c.Work) == 0 && len(c.Projects) == 0 {
			c.Work, c.Projects = l.Selection(doc)
		}
	}
	if c.CreatedAt == "" {
		c.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// The export view: master clone, lens identity, capsule overrides. The
	// master file is never written.
	view := doc.Clone()
	lens.Identity(view, l)
	c.Apply(view)
	view.SetLanguage(c.Language)

	refs := view.Refs()
	letter := c.Cover
	letter.Company = nonEmpty(letter.Company, c.Company)
	letter.Role = nonEmpty(letter.Role, c.Role)
	// A greeting nobody edited follows the application language; one a human
	// typed is left exactly as written.
	if letter.Salutation == "" || letter.Salutation == defaultSalutation("de") || letter.Salutation == defaultSalutation("en") {
		letter.Salutation = defaultSalutation(c.Language)
	}
	c.Salutation = letter.Salutation
	if letter.Opening == "" {
		points := o.Points
		if len(points) == 0 {
			points = seedPoints(view)
		}
		sa := nonEmpty(c.Salutation, letter.Salutation)
		if sa == "" {
			sa = defaultSalutation(c.Language)
		}
		letter = coverletter.Scaffold(coverletter.ScaffoldInput{
			Company:    c.Company,
			Role:       c.Role,
			Salutation: sa,
			LensName:   l.Name,
			Points:     points,
		})
		letter.SourceLang = nonEmpty(letter.SourceLang, "en")
	}
	letter.Normalize()
	// Keep the source-language letter: the capsule stores what a human edits,
	// not the German render, so a rebuild is deterministic. Body is copied
	// because Apply rewrites the strings in place and slices are shared.
	sourceLetter := letter
	sourceLetter.Body = append([]string(nil), letter.Body...)
	refs = append(refs, letter.Refs()...)

	// Fill missing German before rendering, so the review deck and the PDF
	// agree. The cache is keyed by content and saved per item.
	var syncErr error
	if c.Language == "de" && !o.SkipSync && letter.SourceLang != "de" {
		_, syncErr = cache.Sync(refs, i18n.SyncOptions{
			Translator: o.Translator,
			Glossary:   glossary,
			Delay:      o.Delay,
			Progress: func(done, total int, ref resume.Ref, de string, err error) {
				if err != nil {
					o.logf("translate warning: %s: %v", ref.Path, err)
				}
			},
			Save: cache.Save,
		})
	}

	dir := c.Dir(o.Root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if _, err := c.Save(o.Root); err != nil {
		return nil, err
	}

	report := &Report{CapsuleDir: dir, Lang: c.Language, Stats: cache.StatsFor(refs)}
	i18n.Apply(view, cache, c.Language)
	letter.Apply(cache.Lookup)
	if c.Language != "en" {
		report.Translated, report.Missing = cache.Coverage(refs)
	}

	assets, assetWarnings := pdf.ResolveAssets(view, o.Root, !o.NoNetwork)
	report.Warnings = append(report.Warnings, assetWarnings...)
	sigSrc, sigFile := "", ""
	if !o.NoSignature {
		sigSrc, sigFile = signaturePaths(o.Root, assets)
		if sigSrc == "" {
			// A signature path in the resume that resolves to nothing is easy to
			// miss: the PDF is valid, it just has no mark. Say so.
			if want, ok := view.GetString("basics.signature"); ok && strings.TrimSpace(want) != "" {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("signature %q not found; the PDFs carry no signature", want))
			}
		}
	}

	engine := pdf.NewEngine(o.Root)
	version, err := engine.Check()
	if err != nil {
		return nil, err
	}
	if version != "" {
		o.logf("pdfcpu %s", version)
	}

	props := []string{
		"Language = " + c.Language,
		"Lens = " + c.Lens,
		"Company = " + c.Company,
		"SyncedTask = workbench-build",
		"BuildTimestamp = " + time.Now().UTC().Format(time.RFC3339),
	}

	cvPath := filepath.Join(dir, c.Artifact("cv"))
	cvLayoutPath := filepath.Join(dir, "layout_cv.json")
	cvFit := layout.Compile(view, layout.Options{
		Lang:    c.Language,
		Guides:  o.Guides,
		Budget:  o.Budget,
		Density: o.Density,
		SigSrc:  sigSrc,
		SigFile: sigFile,
	})
	if !cvFit.Fits {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"CV needs %d pages at the densest preset (%s); trim a bullet or raise --budget",
			cvFit.Pages, cvFit.Density.ID))
	}
	if err := engine.Create(cvFit.Spec, cvLayoutPath, cvPath); err != nil {
		return nil, err
	}
	if pages, err := engine.Pages(cvPath); err == nil {
		report.Pages = pages
	} else {
		report.Pages = cvFit.Pages
	}
	report.Preset = cvFit.Density.ID
	report.Fitted = cvFit.Fits
	report.Layout = cvLayoutPath
	report.CV = cvPath

	if err := atomicfile.WriteJSON(filepath.Join(dir, c.Artifact("rendered")), view.Root); err != nil {
		return nil, err
	}
	if err := engine.Props(cvPath, props); err != nil {
		return nil, err
	}
	cvAttachments := []string{c.FilePath(o.Root), cvLayoutPath, filepath.Join(dir, c.Artifact("rendered"))}
	if err := engine.Attach(cvPath, cvAttachments...); err != nil {
		return nil, err
	}
	artifacts := []string{filepath.Base(cvPath), c.Artifact("rendered"), filepath.Base(cvLayoutPath)}

	if !o.NoCover {
		letterPath := filepath.Join(dir, c.Artifact("letter"))
		letterLayoutPath := filepath.Join(dir, "layout_letter.json")
		letterJSON := filepath.Join(dir, c.Artifact("letterjson"))
		payload := coverletter.BuildPayload(basicsMap(view), letter, c.Language, formatDate(time.Now(), c.Language))
		if err := atomicfile.WriteJSON(letterJSON, payload); err != nil {
			return nil, err
		}
		letterFit := layout.CompileLetter(view, letter, layout.Options{
			Lang:    c.Language,
			Guides:  o.Guides,
			Budget:  1,
			SigSrc:  sigSrc,
			SigFile: sigFile,
		})
		if !letterFit.Fits {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"cover letter is %d pages; trim a paragraph", letterFit.Pages))
		}
		if err := engine.Create(letterFit.Spec, letterLayoutPath, letterPath); err != nil {
			return nil, err
		}
		if err := engine.Props(letterPath, props); err != nil {
			return nil, err
		}
		if err := engine.Attach(letterPath, c.FilePath(o.Root), letterLayoutPath, letterJSON); err != nil {
			return nil, err
		}
		report.Letter = letterPath
		report.LayoutLetter = letterLayoutPath
		artifacts = append(artifacts, filepath.Base(letterPath), filepath.Base(letterLayoutPath), filepath.Base(letterJSON))

		parts := []string{letterPath, cvPath}
		if cert := attachmentsCert(o.Root); cert != "" {
			parts = append(parts, cert)
		}
		pkg := filepath.Join(dir, c.Artifact("package"))
		if err := engine.Merge(pkg, parts); err != nil {
			return nil, err
		}
		report.Package = pkg
		artifacts = append(artifacts, filepath.Base(pkg))
		o.logf("package %s (%d documents)", filepath.Base(pkg), len(parts))
	}

	if o.Optimize {
		for _, p := range []string{cvPath, report.Letter} {
			if p == "" {
				continue
			}
			if err := engine.Optimize(p); err != nil {
				report.Warnings = append(report.Warnings, "optimize "+filepath.Base(p)+": "+err.Error())
			}
		}
	}

	if copies, err := exportDist(o.Root, c, report); err != nil {
		report.Warnings = append(report.Warnings, "dist: "+err.Error())
	} else {
		report.Dist = copies
	}

	rec := capsule.BuildRecord{
		At:        time.Now().UTC().Format(time.RFC3339),
		Lang:      c.Language,
		Pages:     report.Pages,
		Preset:    report.Preset,
		Artifacts: artifacts,
	}
	c.Cover = sourceLetter
	c.Salutation = sourceLetter.Salutation
	c.AddBuild(rec)
	if _, err := c.Save(o.Root); err != nil {
		return nil, err
	}

	if syncErr != nil {
		report.Warnings = append(report.Warnings,
			"translation was incomplete: "+syncErr.Error())
	}
	if o.ReloadMupdf {
		pdf.Reload(o.logf)
	}
	return report, nil
}

// defaultSalutation keeps the letter usable when nobody typed a greeting.
func defaultSalutation(lang string) string {
	if lang == "de" {
		return "Sehr geehrte Damen und Herren,"
	}
	return "Dear Hiring Manager,"
}

// seedPoints turns the selected CV bullets into letter paragraphs. A headless
// build then produces a real letter from the same decisions as the CV.
func seedPoints(view *resume.Doc) []string {
	var points []string
	for _, item := range view.WorkItems() {
		for _, h := range item.Highlights {
			points = append(points, h)
			if len(points) >= 2 {
				return points
			}
		}
	}
	for _, item := range view.ProjectItems() {
		for _, h := range item.Highlights {
			points = append(points, h)
			if len(points) >= 3 {
				return points
			}
		}
	}
	if len(points) == 0 {
		points = append(points, "The attached CV lists the systems I have built and the results they produced.")
	}
	return points
}

func basicsMap(doc *resume.Doc) map[string]any {
	if b, ok := doc.Root["basics"].(map[string]any); ok {
		return b
	}
	return map[string]any{}
}

func attachmentsCert(root string) string {
	p := filepath.Join(root, "assets", "zeugnisse.pdf")
	if st, err := os.Stat(p); err == nil && st.Size() > 0 {
		return p
	}
	return ""
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

var deMonths = [...]string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}

// formatDate renders the letter date in the target language.
func formatDate(t time.Time, lang string) string {
	if lang == "de" {
		return fmt.Sprintf("%d. %s %d", t.Day(), deMonths[t.Month()-1], t.Year())
	}
	return t.Format("2 January 2006")
}

// signaturePaths resolves the signature image for the layout compiler. pdfcpu
// resolves image paths against its working directory (the repo root), so the
// compiler needs the file both as a root-relative src and as a real path.
func signaturePaths(root string, assets map[string]string) (src, file string) {
	p := assets["signature"]
	if p == "" {
		return "", ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", ""
	}
	relPath, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", ""
	}
	return filepath.ToSlash(relPath), abs
}

// exportDist mirrors the capsule artifacts into dist/ under the names the
// pipeline documents use. dist/ is disposable: the capsule stays authoritative.
func exportDist(root string, c *capsule.Capsule, r *Report) ([]string, error) {
	dist := filepath.Join(root, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return nil, err
	}
	stem, lang := c.FileStem, c.Language
	if stem == "" {
		stem = "Resume"
	}
	pairs := [][2]string{
		{r.CV, "CV_" + stem + "_" + lang + ".pdf"},
		{r.Layout, "pdfcpu_cv_layout_" + lang + ".json"},
	}
	if r.Letter != "" {
		pairs = append(pairs, [2]string{r.Letter, "Cover_Letter_" + stem + "_" + lang + ".pdf"})
	}
	if r.LayoutLetter != "" {
		pairs = append(pairs, [2]string{r.LayoutLetter, "pdfcpu_letter_layout_" + lang + ".json"})
	}
	if r.Package != "" {
		pairs = append(pairs, [2]string{r.Package, "Bewerbungsmappe_" + stem + "_" + lang + ".pdf"})
	}
	var out []string
	for _, p := range pairs {
		if p[0] == "" {
			continue
		}
		dst := filepath.Join(dist, p[1])
		if err := copyFile(p[0], dst); err != nil {
			return out, err
		}
		out = append(out, dst)
	}
	return out, nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
