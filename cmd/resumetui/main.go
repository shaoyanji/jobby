package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Config struct {
	Resume   string
	Cache    string
	Loadouts string
	Template string
	OutDir   string
	Lang     string
	Loadout  string
}

func main() {
	var (
		syncI18n bool
		export   string
		out      string
		noTUI    bool
	)
	cfg := Config{}
	flag.StringVar(&cfg.Resume, "resume", "resume.json", "JSON Resume source of truth")
	flag.StringVar(&cfg.Cache, "cache", "i18n_cache.json", "SHA-256 translation sidecar")
	flag.StringVar(&cfg.Loadouts, "loadouts", "loadouts.json", "saved loadouts")
	flag.StringVar(&cfg.Template, "template", "templates/resume.typ", "Typst template")
	flag.StringVar(&cfg.OutDir, "out-dir", "dist", "build output directory")
	flag.StringVar(&cfg.Lang, "lang", "", "active language (en|de)")
	flag.StringVar(&cfg.Loadout, "loadout", "", "loadout name (defaults to the active one)")
	flag.BoolVar(&syncI18n, "sync-i18n", false, "translate uncached strings and exit")
	flag.StringVar(&export, "export-lang", "", "export the build JSON for a language and exit")
	flag.StringVar(&out, "out", "", "export destination (default dist/active-<lang>.json)")
	flag.BoolVar(&noTUI, "no-tui", false, "export with the active language instead of starting the TUI")
	flag.Parse()

	switch {
	case syncI18n:
		if err := runSync(cfg); err != nil {
			die(err)
		}
	case export != "" || noTUI:
		lang := export
		if lang == "" {
			lang = cfg.Lang
		}
		if err := runExport(cfg, lang, cfg.Loadout, out); err != nil {
			die(err)
		}
	default:
		m, err := newModel(cfg)
		if err != nil {
			die(err)
		}
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			die(err)
		}
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "resumetui:", err)
	os.Exit(1)
}

func runSync(cfg Config) error {
	r, err := LoadResume(cfg.Resume)
	if err != nil {
		return err
	}
	c, err := LoadCache(cfg.Cache)
	if err != nil {
		return err
	}
	refs := r.Translatable()
	n, errs := c.Sync(refs, func(done, total int, path string) {
		fmt.Fprintf(os.Stderr, "\r[%d/%d] %-32s", done, total, path)
	})
	if n > 0 {
		fmt.Fprintln(os.Stderr)
	}
	if err := c.Save(cfg.Cache); err != nil {
		return err
	}
	verified, auto, missing := c.Coverage(refs)
	fmt.Printf("translated %d · cache %d · verified %d · auto %d · missing %d\n",
		n, len(c), verified, auto, missing)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "warn:", e)
	}
	return nil
}

func runExport(cfg Config, lang, loadoutName, out string) error {
	r, err := LoadResume(cfg.Resume)
	if err != nil {
		return err
	}
	lf, err := LoadLoadouts(cfg.Loadouts)
	if err != nil {
		return err
	}
	if loadoutName == "" {
		loadoutName = lf.Active
	}
	if loadoutName == "" {
		loadoutName = "default"
	}
	if l, ok := lf.Get(loadoutName); ok {
		ApplyLoadout(r, l)
		if lang == "" && l.Language != "" {
			lang = l.Language
		}
	}
	if lang == "" {
		lang = "en"
	}
	c, err := LoadCache(cfg.Cache)
	if err != nil {
		return err
	}
	translated, missing := r.ApplyLanguage(lang, c)
	id := stampSync(r, loadoutName, lang)
	if out == "" {
		out = filepath.Join(cfg.OutDir, "active-"+lang+".json")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := r.Save(out); err != nil {
		return err
	}
	fmt.Printf("exported %s\n  loadout=%s lang=%s synced_task=%s translated=%d missing=%d\n",
		out, loadoutName, lang, id, translated, missing)
	return nil
}

// stampSync records the build in meta so a PDF can be traced back to the exact
// loadout and language that produced it.
func stampSync(r *Resume, loadout, lang string) string {
	now := time.Now().UTC()
	if loadout == "" {
		loadout = "default"
	}
	id := now.Format("20060102-150405") + "-" + loadout + "-" + lang
	var tasks []any
	switch v := r.Meta["synced_tasks"].(type) {
	case []any:
		tasks = v
	case []string:
		for _, s := range v {
			tasks = append(tasks, s)
		}
	}
	tasks = append(tasks, id)
	r.Meta["synced_tasks"] = tasks
	r.Meta["last_synced"] = now.Format(time.RFC3339)
	return id
}
