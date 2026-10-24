// Command workbench turns one master resume into per-application capsules:
// a fitted CV, a matching cover letter, and one merged package, with the state
// embedded in the PDFs.
//
// The TUI is a convenience; every action has a flag so a build can run
// unattended:
//
//	workbench build --lens=ai --lang=de --company=TestGmbH
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shaoyanji/jobby/internal/capsule"
	"github.com/shaoyanji/jobby/internal/i18n"
	"github.com/shaoyanji/jobby/internal/layout"
	"github.com/shaoyanji/jobby/internal/workbench"
)

const version = "0.1.0"

const usageText = `workbench — application capsules for a JSON Resume

Usage:
  workbench [tui] [flags]
  workbench build --company NAME [flags]
  workbench new --company NAME [flags]
  workbench list
  workbench import-pdf <file.pdf>
  workbench version

Commands:
  tui           Interactive menu: new/switch capsule, tailor, letter, export
  build         Render CV + cover letter + package for one capsule
  new           Create a capsule (and its cover-letter draft) without building
  list          List capsules under applications/
  import-pdf    Restore a capsule from the JSON embedded in an old PDF

Common flags:
  --root DIR        Repository root (default ".")
  --budget N        Page budget for the CV (default 2)
  --mock            Offline translation backend (marks text with [de])
  --verbose         Print every pipeline step

Build flags:
  --company NAME    Target company (required unless --capsule is given)
  --capsule DIR     Build an existing capsule folder
  --lens ID         Lens to use (ai, energy, consulting); overrides the capsule
  --lang en|de      Application language
  --role TITLE      Role title for the cover letter
  --salutation TEXT Greeting; defaults per language
  --points "a;b"    Cover-letter talking points (default: the lens highlights)
  --no-cover        Skip the cover letter and package
  --no-stamp        Do not stamp assets/signature.png
  --no-optimize     Skip pdfcpu optimize
  --no-sync         Do not call trans; render from the cache only
`

type common struct {
	root    string
	budget  int
	mock    bool
	verbose bool
}

func (c *common) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.root, "root", ".", "repository root")
	fs.IntVar(&c.budget, "budget", 2, "CV page budget")
	fs.BoolVar(&c.mock, "mock", false, "offline translation backend")
	fs.BoolVar(&c.verbose, "verbose", false, "print every pipeline step")
}

func (c *common) rootDir() string {
	if c.root == "" {
		return "."
	}
	return c.root
}

func (c *common) translator() (i18n.Translator, bool) {
	if c.mock {
		return i18n.Mock{}, true
	}
	tr := i18n.NewTransCLI()
	return tr, tr.Available()
}

func main() {
	if len(os.Args) < 2 {
		if err := runTUIWith(nil); err != nil {
			fmt.Fprintf(os.Stderr, "workbench: %v\n", err)
			os.Exit(1)
		}
		return
	}
	cmd := os.Args[1]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		fmt.Print(usageText)
		return
	}
	if cmd == "version" || cmd == "--version" {
		fmt.Printf("workbench %s\n", version)
		return
	}

	var err error
	switch cmd {
	case "tui":
		err = runTUIWith(os.Args[2:])
	case "build":
		err = cmdBuild(os.Args[2:])
	case "new":
		err = cmdNew(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "import-pdf", "import":
		err = cmdImport(os.Args[2:])
	default:
		// Flags without a subcommand mean "open the TUI with these settings".
		if strings.HasPrefix(cmd, "-") {
			err = runTUIWith(os.Args[1:])
		} else {
			fmt.Fprintf(os.Stderr, "workbench: unknown command %q\n\n%s", cmd, usageText)
			os.Exit(2)
		}
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "workbench: %v\n", err)
		os.Exit(1)
	}
}

func runTUIWith(args []string) error {
	var c common
	fs := flag.NewFlagSet("workbench", flag.ContinueOnError)
	c.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := &workbench.Session{Root: c.rootDir(), Budget: c.budget, Mock: c.mock}
	return s.Run()
}

type buildFlags struct {
	common
	company    string
	capsuleDir string
	lens       string
	lang       string
	role       string
	salutation string
	points     string
	noCover    bool
	noStamp    bool
	guides     bool
	density    string
	noOptimize bool
	noSync     bool
	reload     bool
}

func (b *buildFlags) bind(fs *flag.FlagSet) {
	b.common.bind(fs)
	fs.StringVar(&b.company, "company", "", "target company")
	fs.StringVar(&b.capsuleDir, "capsule", "", "existing capsule folder to build")
	fs.StringVar(&b.lens, "lens", "", "lens id (ai, energy, consulting)")
	fs.StringVar(&b.lang, "lang", "", "application language: en or de")
	fs.StringVar(&b.role, "role", "", "role title for the cover letter")
	fs.StringVar(&b.salutation, "salutation", "", "cover letter greeting")
	fs.StringVar(&b.points, "points", "", "cover-letter talking points, separated by ; or newlines")
	fs.BoolVar(&b.noCover, "no-cover", false, "skip the cover letter and package")
	fs.BoolVar(&b.noStamp, "no-stamp", false, "do not draw the signature image")
	fs.BoolVar(&b.guides, "guides", false, "draw pdfcpu alignment guides and the content box")
	fs.StringVar(&b.density, "density", "", "force a typography preset (10.0, 9.5, 9.0) instead of auto-fitting")
	fs.BoolVar(&b.noOptimize, "no-optimize", false, "skip pdfcpu optimize")
	fs.BoolVar(&b.noSync, "no-sync", false, "do not call trans; use the cache only")
	fs.BoolVar(&b.reload, "reload-mupdf", false, "SIGHUP a running mupdf after the build")
}

func cmdBuild(args []string) error {
	var b buildFlags
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	b.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := b.rootDir()

	var c *capsule.Capsule
	if b.capsuleDir != "" {
		loaded, err := capsule.Load(b.capsuleDir)
		if err != nil {
			return err
		}
		c = loaded
	} else {
		if strings.TrimSpace(b.company) == "" {
			return errors.New("build needs --company or --capsule")
		}
		existing, err := capsule.Find(root, b.company)
		if err != nil {
			return err
		}
		if existing != nil {
			c = existing
			fmt.Fprintf(os.Stderr, "reusing %s\n", c.Dir(root))
		} else {
			created, _, err := workbench.Draft(root, b.company, b.role, b.salutation, b.lang, b.lens, splitPoints(b.points))
			if err != nil {
				return err
			}
			c = created
			fmt.Fprintf(os.Stderr, "created %s\n", c.Dir(root))
		}
	}

	if b.company != "" && c.Company != b.company {
		// A capsule is bound to its company folder; renaming would move it.
		fmt.Fprintf(os.Stderr, "warning: capsule belongs to %q, keeping it\n", c.Company)
	}
	if b.role != "" {
		c.Role = b.role
	}
	if b.salutation != "" {
		c.Salutation = b.salutation
	}
	if b.lang != "" {
		c.Language = b.lang
	}
	if b.lens != "" {
		c.Lens = b.lens
	}

	opt := workbench.Options{
		Root:        root,
		Capsule:     c,
		LensID:      b.lens,
		Lang:        b.lang,
		Points:      splitPoints(b.points),
		Budget:      b.budget,
		Guides:      b.guides,
		Density:     densityPreset(b.density),
		NoSignature: b.noStamp,
		Optimize:    !b.noOptimize,
		NoCover:     b.noCover,
		SkipSync:    b.noSync,
		ReloadMupdf: b.reload,
	}
	if b.verbose {
		opt.Log = func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	}
	if b.mock {
		opt.Translator = i18n.Mock{}
	} else if tr, ok := b.translator(); ok {
		opt.Translator = tr
	} else {
		fmt.Fprintln(os.Stderr, "warning: translate-shell (trans) is not on PATH; using cached translations only")
		opt.SkipSync = true
	}

	rep, err := workbench.Build(opt)
	if err != nil {
		return err
	}
	for _, w := range rep.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	fit := fmt.Sprintf("fitted to %s · %s", pages(rep.Pages), rep.Preset)
	if !rep.Fitted {
		fit = fmt.Sprintf("%s at the densest preset (%s)", pages(rep.Pages), rep.Preset)
	}
	fmt.Printf("capsule  %s\n", rep.CapsuleDir)
	fmt.Printf("cv       %s · %s\n", filepath.Base(rep.CV), fit)
	if rep.Layout != "" {
		fmt.Printf("layout   %s\n", filepath.Base(rep.Layout))
	}
	if rep.Letter != "" {
		fmt.Printf("letter   %s\n", filepath.Base(rep.Letter))
	}
	if rep.Package != "" {
		fmt.Printf("package  %s\n", filepath.Base(rep.Package))
	}
	if len(rep.Dist) > 0 {
		fmt.Printf("dist     %d files\n", len(rep.Dist))
	}
	if rep.Lang == "de" {
		fmt.Printf("i18n     %d translated · %d missing · %s\n",
			rep.Translated, rep.Missing, rep.Stats.String())
	} else {
		fmt.Println("i18n     english source (no translation needed)")
	}
	return nil
}

// pages keeps the summary readable: "1 page", not "1 page(s)".
func pages(n int) string {
	if n == 1 {
		return "1 page"
	}
	return fmt.Sprintf("%d pages", n)
}

// densityPreset maps --density onto the compiler's ladder.
func densityPreset(id string) *layout.Density {
	id = strings.TrimSuffix(strings.TrimSpace(id), "pt")
	if id == "" {
		return nil
	}
	for i := range layout.Densities {
		d := layout.Densities[i]
		if strings.TrimSuffix(d.ID, "pt") == id {
			return &d
		}
	}
	return nil
}

func cmdNew(args []string) error {
	var b buildFlags
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	b.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(b.company) == "" {
		return errors.New("new needs --company")
	}
	_, dir, err := workbench.Draft(b.rootDir(), b.company, b.role, b.salutation, b.lang, b.lens, splitPoints(b.points))
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", dir)
	return nil
}

func cmdList(args []string) error {
	var c common
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	c.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	sums, err := capsule.List(c.rootDir())
	if err != nil {
		return err
	}
	if len(sums) == 0 {
		fmt.Println("no capsules under applications/")
		return nil
	}
	for _, s := range sums {
		role := s.Role
		if role == "" {
			role = "-"
		}
		fmt.Printf("%-22s  %-20s  %-10s  %s\n",
			filepath.Base(s.Dir), s.Company, s.Language, role)
	}
	return nil
}

func cmdImport(args []string) error {
	var c common
	fs := flag.NewFlagSet("import-pdf", flag.ContinueOnError)
	c.bind(fs)
	pdf := fs.String("pdf", "", "PDF to recover")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := *pdf
	if path == "" && fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	if path == "" {
		return errors.New("usage: workbench import-pdf <file.pdf>")
	}
	dir, err := workbench.ImportPDF(c.rootDir(), path)
	if err != nil {
		return err
	}
	fmt.Println(dir)
	return nil
}

// splitPoints accepts "a;b" or newline-separated talking points and drops
// empty entries.
func splitPoints(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ';' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
