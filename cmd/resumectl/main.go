// Command resumectl is the headless core of the jobby resume pipeline, with an
// optional interactive layer.
//
// Every action works as a flag so it can be scripted, piped through jq, or
// driven from Taskfile.yml. The interactive forms (huh) are a convenience on
// top, never the only way to change state.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shaoyanji/jobby/internal/export"
	"github.com/shaoyanji/jobby/internal/i18n"
	"github.com/shaoyanji/jobby/internal/loadout"
	"github.com/shaoyanji/jobby/internal/pdf"
	"github.com/shaoyanji/jobby/internal/resume"
	"github.com/shaoyanji/jobby/internal/tui"
)

const version = "1.0.0"

const usageText = `resumectl — bilingual JSON Resume pipeline

Usage:
  resumectl <command> [flags]

Commands:
  i18n-sync      Translate English strings that have no cached German yet
  i18n-review    Interactively verify or rewrite German translations
  stats          Report translation coverage
  loadout-edit   Interactively build a named highlight loadout
  render-json    Export the loadout + language view (lossless)
  build-pdf      Export, compile with typst, post-process with pdfcpu
  mappe          Merge cover letter + German CV + references
  unpack         Extract the JSON snapshot embedded in a PDF
  tui            Menu over the commands above
  version        Print the version

Common flags:
  --root DIR         Repository root (default ".")
  --resume FILE      Source document (default <root>/resume.json)
  --cache FILE       Translation sidecar (default <root>/i18n_cache.json)
  --loadouts FILE    Loadout store (default <root>/loadouts.json)
  --glossary FILE    Term glossary (default <root>/glossary.json)

Run "resumectl <command> -h" for a command's own flags.
`

type common struct {
	root         string
	resumePath   string
	cachePath    string
	loadoutsPath string
	glossaryPath string
	verbose      bool
}

func (c *common) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.root, "root", ".", "repository root")
	fs.StringVar(&c.resumePath, "resume", "", "source resume JSON")
	fs.StringVar(&c.cachePath, "cache", "", "translation cache")
	fs.StringVar(&c.loadoutsPath, "loadouts", "", "loadout store")
	fs.StringVar(&c.glossaryPath, "glossary", "", "glossary JSON")
	fs.BoolVar(&c.verbose, "v", false, "verbose progress output")
}

func (c *common) rootDir() string {
	if c.root == "" {
		return "."
	}
	return c.root
}

func (c *common) resumeFile() string {
	if c.resumePath != "" {
		return c.resumePath
	}
	return filepath.Join(c.rootDir(), "resume.json")
}

func (c *common) cacheFile() string {
	if c.cachePath != "" {
		return c.cachePath
	}
	return filepath.Join(c.rootDir(), "i18n_cache.json")
}

func (c *common) loadoutsFile() string {
	if c.loadoutsPath != "" {
		return c.loadoutsPath
	}
	return filepath.Join(c.rootDir(), "loadouts.json")
}

func (c *common) glossaryFile() string {
	if c.glossaryPath != "" {
		return c.glossaryPath
	}
	return filepath.Join(c.rootDir(), "glossary.json")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	cmd := os.Args[1]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		fmt.Print(usageText)
		return
	}
	if cmd == "version" || cmd == "--version" {
		fmt.Printf("resumectl %s\n", version)
		return
	}

	var err error
	switch cmd {
	case "i18n-sync":
		err = cmdSync(os.Args[2:])
	case "i18n-review":
		err = cmdReview(os.Args[2:])
	case "stats":
		err = cmdStats(os.Args[2:])
	case "loadout-edit":
		err = cmdLoadoutEdit(os.Args[2:])
	case "render-json":
		err = cmdRender(os.Args[2:])
	case "build-pdf":
		err = cmdBuildPDF(os.Args[2:])
	case "mappe":
		err = cmdMappe(os.Args[2:])
	case "unpack":
		err = cmdUnpack(os.Args[2:])
	case "tui":
		err = cmdTUI(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "resumectl: unknown command %q\n\n%s", cmd, usageText)
		os.Exit(2)
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "resumectl: %v\n", err)
		os.Exit(1)
	}
}

func cmdSync(args []string) error {
	var (
		c      common
		dryRun bool
		mock   bool
		delay  time.Duration
	)
	fs := flag.NewFlagSet("i18n-sync", flag.ContinueOnError)
	c.bind(fs)
	fs.BoolVar(&dryRun, "dry-run", false, "report what would be translated, write nothing")
	fs.BoolVar(&mock, "mock", false, "use the offline placeholder translator")
	fs.DurationVar(&delay, "delay", 250*time.Millisecond, "pause between network calls")
	if err := fs.Parse(args); err != nil {
		return err
	}

	doc, err := resume.Load(c.resumeFile())
	if err != nil {
		return err
	}
	cache, err := i18n.Load(c.cacheFile())
	if err != nil {
		return err
	}
	g, err := i18n.LoadGlossary(c.glossaryFile())
	if err != nil {
		return err
	}

	translator := i18n.Translator(i18n.NewTransCLI())
	if mock {
		translator = i18n.Mock{}
	} else if tr, ok := translator.(i18n.TransCLI); ok && !tr.Available() {
		return fmt.Errorf("translate-shell (trans) is not on PATH; run inside `nix develop` or pass --mock")
	}

	refs := doc.Refs()
	before := cache.StatsFor(refs)
	progress := func(done, total int, ref resume.Ref, de string, err error) {
		if !c.verbose {
			return
		}
		switch {
		case err != nil:
			fmt.Printf("  [warn] %-28s %v\n", ref.Path, err)
		case dryRun:
			fmt.Printf("  [would] %-28s %s\n", ref.Path, clip(de, 60))
		default:
			fmt.Printf("  [%d/%d] %-28s %s\n", done, total, ref.Path, clip(de, 60))
		}
	}

	fmt.Printf("sync: %d unique strings, %d need translation (%s)\n",
		before.Total, before.Missing, translator.Name())
	stats, syncErr := cache.Sync(refs, i18n.SyncOptions{
		Translator: translator,
		Glossary:   g,
		Delay:      delay,
		DryRun:     dryRun,
		Progress:   progress,
		Save:       cache.Save,
	})
	orphans := cache.MarkOrphans(refs)
	if !dryRun {
		if err := cache.Save(); err != nil {
			return err
		}
	}
	fmt.Printf("i18n: %s\n", stats.String())
	if orphans > 0 {
		fmt.Printf("i18n: %d entr(y/ies) newly orphaned (kept, marked orphaned)\n", orphans)
	}
	return syncErr
}

func cmdReview(args []string) error {
	var (
		c      common
		prefix string
	)
	fs := flag.NewFlagSet("i18n-review", flag.ContinueOnError)
	c.bind(fs)
	fs.StringVar(&prefix, "path", "", "only review entries whose path_hint starts with this prefix")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cache, err := i18n.Load(c.cacheFile())
	if err != nil {
		return err
	}
	return tui.Review(cache, prefix)
}

func cmdStats(args []string) error {
	var c common
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	c.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	doc, err := resume.Load(c.resumeFile())
	if err != nil {
		return err
	}
	cache, err := i18n.Load(c.cacheFile())
	if err != nil {
		return err
	}
	fmt.Println(cache.StatsFor(doc.Refs()).String())
	for _, e := range cache.EntriesView() {
		if e.Orphaned {
			fmt.Printf("  orphaned  %-24s %s\n", e.PathHint, clip(e.EN, 70))
		}
	}
	return nil
}

func cmdLoadoutEdit(args []string) error {
	var (
		c       common
		profile string
	)
	fs := flag.NewFlagSet("loadout-edit", flag.ContinueOnError)
	c.bind(fs)
	fs.StringVar(&profile, "profile", "", "loadout name to edit or create")
	if err := fs.Parse(args); err != nil {
		return err
	}
	doc, err := resume.Load(c.resumeFile())
	if err != nil {
		return err
	}
	file, err := loadout.Load(c.loadoutsFile())
	if err != nil {
		return err
	}
	before := doc.MetaString("last_synced")

	l, err := tui.EditLoadouts(doc, file, profile)
	if err != nil {
		return err
	}
	if err := file.Save(c.loadoutsFile()); err != nil {
		return err
	}
	if doc.MetaString("last_synced") != before {
		if err := doc.Save(c.resumeFile()); err != nil {
			return err
		}
	}
	fmt.Printf("loadout %q saved (%d work, %d project keys); persona %q\n",
		l.Name, len(l.Work), len(l.Projects), l.Label)
	return nil
}

func cmdRender(args []string) error {
	var (
		c       common
		lang    string
		profile string
		out     string
	)
	fs := flag.NewFlagSet("render-json", flag.ContinueOnError)
	c.bind(fs)
	fs.StringVar(&lang, "lang", "en", "en or de")
	fs.StringVar(&profile, "profile", "", "loadout name")
	fs.StringVar(&out, "out", "-", "output path, or - for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	res, err := export.Run(export.Request{
		ResumePath:   c.resumeFile(),
		CachePath:    c.cacheFile(),
		LoadoutsPath: c.loadoutsFile(),
		Lang:         lang,
		Profile:      profile,
		SaveLoadouts: true,
	})
	if err != nil {
		return err
	}
	written, err := res.Write(out, os.Stdout)
	if err != nil {
		return err
	}
	if written != "-" {
		fmt.Fprintf(os.Stderr, "wrote %s (%s: %d translated, %d missing)\n", written, lang, res.Translated, res.Missing)
		if res.Missing > 0 {
			fmt.Fprintf(os.Stderr, "hint: %d string(s) fell back to English; run `resumectl i18n-sync` then `i18n-review`\n", res.Missing)
		}
	}
	return nil
}

type buildFlags struct {
	common
	lang       string
	profile    string
	out        string
	jsonOut    string
	template   string
	stampSig   bool
	attachJSON bool
	reload     bool
	noOptimize bool
	noNetwork  bool
	pageBudget int
}

func (b *buildFlags) bind(fs *flag.FlagSet) {
	b.common.bind(fs)
	fs.StringVar(&b.lang, "lang", "en", "en or de")
	fs.StringVar(&b.profile, "profile", "default", "loadout name")
	fs.StringVar(&b.out, "out", "", "PDF output path (default dist/resume-<lang>.pdf)")
	fs.StringVar(&b.jsonOut, "json-out", "", "exported JSON path (default dist/resume-<lang>.json)")
	fs.StringVar(&b.template, "template", "", "typst template (default templates/resume.typ)")
	fs.BoolVar(&b.stampSig, "stamp-sig", false, "stamp assets/signature.* on the last page")
	fs.BoolVar(&b.attachJSON, "attach-json", false, "embed the exported JSON in the PDF")
	fs.BoolVar(&b.reload, "reload-mupdf", false, "SIGHUP a running mupdf after the build")
	fs.BoolVar(&b.noOptimize, "no-optimize", false, "skip pdfcpu optimize")
	fs.BoolVar(&b.noNetwork, "no-network", false, "never download remote avatar/signature images")
	fs.IntVar(&b.pageBudget, "page-budget", 2, "warn above this page count")
}

func (b *buildFlags) options() pdf.Options {
	return pdf.Options{
		Root:         b.rootDir(),
		Lang:         b.lang,
		Profile:      b.profile,
		ResumePath:   b.resumePath,
		CachePath:    b.cachePath,
		LoadoutsPath: b.loadoutsPath,
		Template:     b.template,
		OutPDF:       b.out,
		OutJSON:      b.jsonOut,
		StampSig:     b.stampSig,
		AttachJSON:   b.attachJSON,
		ReloadMupdf:  b.reload,
		SkipOptimize: b.noOptimize,
		PageBudget:   b.pageBudget,
		NoNetwork:    b.noNetwork,
		Log: func(format string, args ...any) {
			fmt.Printf(format+"\n", args...)
		},
	}
}

func cmdBuildPDF(args []string) error {
	var b buildFlags
	fs := flag.NewFlagSet("build-pdf", flag.ContinueOnError)
	b.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	res, err := pdf.Build(b.options())
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	return nil
}

func cmdMappe(args []string) error {
	var b buildFlags
	fs := flag.NewFlagSet("mappe", flag.ContinueOnError)
	b.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	opt := b.options()
	opt.Lang = "de"
	opt.Profile = b.profile
	opt.StampSig = b.stampSig
	opt.AttachJSON = true
	if _, err := pdf.Mappe(opt); err != nil {
		return err
	}
	return nil
}

func cmdUnpack(args []string) error {
	var (
		c   common
		dir string
	)
	fs := flag.NewFlagSet("unpack", flag.ContinueOnError)
	c.bind(fs)
	fs.StringVar(&dir, "dir", "", "destination directory (default: a temp dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New("usage: resumectl unpack <file.pdf> [--dir DIR]")
	}
	// Go's flag package stops at the first positional, so accept trailing flags
	// as well: `unpack file.pdf --dir out` is what people type.
	if len(rest) > 1 {
		tail := flag.NewFlagSet("unpack", flag.ContinueOnError)
		tail.StringVar(&dir, "dir", dir, "destination directory")
		if err := tail.Parse(rest[1:]); err != nil {
			return err
		}
	}
	files, err := pdf.Unpack(rest[0], dir, func(format string, args ...any) {
		fmt.Printf(format+"\n", args...)
	})
	if err != nil {
		return err
	}
	for _, f := range files {
		fmt.Println(f)
	}
	return nil
}

func cmdTUI(args []string) error {
	var c common
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	c.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	base := []string{
		"--root", c.rootDir(),
		"--resume", c.resumeFile(),
		"--cache", c.cacheFile(),
		"--loadouts", c.loadoutsFile(),
		"--glossary", c.glossaryFile(),
	}
	for {
		action, err := tui.Menu()
		if err != nil {
			return err
		}
		switch action {
		case tui.ActionQuit:
			return nil
		case tui.ActionReview:
			if err := cmdReview(base); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		case tui.ActionLoadout:
			if err := cmdLoadoutEdit(base); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		case tui.ActionSync:
			if err := cmdSync(base); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		case tui.ActionBuildEN, tui.ActionBuildDE:
			lang := "en"
			if action == tui.ActionBuildDE {
				lang = "de"
			}
			res, err := pdf.Build(pdf.Options{
				Root:        c.rootDir(),
				Lang:        lang,
				Profile:     "default",
				StampSig:    true,
				AttachJSON:  true,
				ReloadMupdf: true,
				Log: func(format string, args ...any) {
					fmt.Printf(format+"\n", args...)
				},
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				continue
			}
			fmt.Printf("built %s\n", res.PDF)
		}
	}
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
