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

	"github.com/shaoyanji/jobby/internal/capsule"
	"github.com/shaoyanji/jobby/internal/coverletter"
	"github.com/shaoyanji/jobby/internal/i18n"
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

	Budget      int
	StampSig    bool
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
	CapsuleDir string
	Lang       string
	CV         string
	Letter     string
	Package    string
	Pages      int
	Preset     string
	Fitted     bool
	Translated int
	Missing    int
	Stats      i18n.Stats
	Warnings   []string
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
	inputs := map[string]string{}
	if p := assets["avatar"]; p != "" {
		inputs["avatar"] = p
	}

	cvPath := filepath.Join(dir, c.Artifact("cv"))
	props := map[string]string{
		"Language":       c.Language,
		"Lens":           c.Lens,
		"Company":        c.Company,
		"Profile":        c.Lens,
		"BuildTimestamp": time.Now().UTC().Format(time.RFC3339),
	}
	cvRes, err := pdf.BuildDocument(pdf.DocOptions{
		Root:        o.Root,
		Template:    filepath.Join(o.Root, "templates", "resume.typ"),
		JSONPath:    filepath.Join(dir, c.Artifact("rendered")),
		PDFPath:     cvPath,
		Doc:         view,
		Inputs:      inputs,
		Attachments: []string{c.FilePath(o.Root)},
		AttachJSON:  true,
		StampSig:    o.StampSig,
		Optimize:    o.Optimize,
		Budget:      o.Budget,
		Properties:  props,
		Log:         o.Log,
	})
	if err != nil {
		return nil, err
	}
	report.CV = cvPath
	report.Pages = cvRes.Pages
	report.Preset = cvRes.Preset.Label
	report.Fitted = cvRes.Fitted

	artifacts := []string{filepath.Base(cvPath), c.Artifact("rendered")}

	if !o.NoCover {
		letterPath := filepath.Join(dir, c.Artifact("letter"))
		payload := coverletter.BuildPayload(
			basicsMap(view), letter, c.Language, formatDate(time.Now(), c.Language))
		letterRes, err := pdf.BuildDocument(pdf.DocOptions{
			Root:        o.Root,
			Template:    filepath.Join(o.Root, "templates", "cover_letter.typ"),
			JSONPath:    filepath.Join(dir, c.Artifact("letterjson")),
			PDFPath:     letterPath,
			Payload:     payload,
			Inputs:      inputs,
			Attachments: []string{c.FilePath(o.Root)},
			AttachJSON:  true,
			StampSig:    o.StampSig,
			Optimize:    o.Optimize,
			Budget:      1,
			Properties:  props,
			Log:         o.Log,
		})
		if err != nil {
			return nil, err
		}
		report.Letter = letterPath
		if !letterRes.Fitted {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("cover letter is %d pages; tighten it or trim a paragraph", letterRes.Pages))
		}
		artifacts = append(artifacts, filepath.Base(letterPath))

		parts := []string{letterPath, cvPath}
		if cert := attachmentsCert(o.Root); cert != "" {
			parts = append(parts, cert)
		}
		pkg := filepath.Join(dir, c.Artifact("package"))
		if err := pdf.Merge(pkg, parts); err != nil {
			return nil, err
		}
		report.Package = pkg
		artifacts = append(artifacts, filepath.Base(pkg))
		o.logf("package %s (%d documents)", filepath.Base(pkg), len(parts))
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
