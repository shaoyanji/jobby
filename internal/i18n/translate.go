package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shaoyanji/jobby/internal/resume"
)

// Translator turns one English string into German.
type Translator interface {
	Translate(en string) (string, error)
	Name() string
}

// TransCLI shells out to translate-shell (`trans -b -no-ansi en:de`).
type TransCLI struct {
	Bin  string
	From string
	To   string
}

// NewTransCLI returns a translator for the en:de pair.
func NewTransCLI() TransCLI { return TransCLI{Bin: "trans", From: "en", To: "de"} }

// Name reports the backend for progress output.
func (t TransCLI) Name() string {
	bin := t.Bin
	if bin == "" {
		bin = "trans"
	}
	return bin
}

// Available reports whether the binary is on PATH.
func (t TransCLI) Available() bool {
	bin := t.Bin
	if bin == "" {
		bin = "trans"
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

// Translate runs one translation. Flags go before the pair so a sentence
// starting with "-" is never parsed as an option.
func (t TransCLI) Translate(en string) (string, error) {
	bin := t.Bin
	if bin == "" {
		bin = "trans"
	}
	from, to := t.From, t.To
	if from == "" {
		from = "en"
	}
	if to == "" {
		to = "de"
	}
	cmd := exec.Command(bin, "-b", "-no-ansi", from+":"+to, "--", en)
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return "", fmt.Errorf("%s: %s", bin, strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("%s: %w", bin, err)
	}
	de := strings.TrimSpace(string(out))
	if de == "" {
		return "", fmt.Errorf("%s returned nothing", bin)
	}
	return de, nil
}

// Mock is the offline backend: it marks the text instead of guessing at it, so
// a dry run still produces a checkable cache shape.
type Mock struct{}

// Name reports the backend for progress output.
func (Mock) Name() string { return "mock" }

// Translate returns the English text with a [de] marker.
func (Mock) Translate(en string) (string, error) {
	return "[de] " + strings.TrimSpace(en), nil
}

// Glossary maps an English term to the German wording that must survive
// machine translation (product names, acronyms, house vocabulary).
type Glossary map[string]string

// LoadGlossary reads glossary.json. A missing file is an empty glossary.
func LoadGlossary(path string) (Glossary, error) {
	g := Glossary{}
	if path == "" {
		return g, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return g, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read glossary: %w", err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return g, nil
	}
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("parse glossary %s: %w", path, err)
	}
	return g, nil
}

// Apply rewrites glossary terms in a translated string. Longer terms win, so
// "eval harnesses" is replaced before "harness".
func (g Glossary) Apply(s string) string {
	if len(g) == 0 || s == "" {
		return s
	}
	keys := make([]string, 0, len(g))
	for k := range g {
		if strings.TrimSpace(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(k) + `\b`)
		if err != nil {
			continue
		}
		s = re.ReplaceAllLiteralString(s, g[k])
	}
	return s
}

// SyncOptions controls a translation pass.
type SyncOptions struct {
	Translator Translator
	Glossary   Glossary
	// Delay is the pause between network calls. translate-shell talks to an
	// unofficial endpoint, so hammering it gets the IP throttled.
	Delay    time.Duration
	DryRun   bool
	Progress func(done, total int, ref resume.Ref, de string, err error)
	// Save is called after every successful item so a crash costs one string,
	// not the whole pass.
	Save func() error
}

// Sync fills in every string that has no translation yet. Human edits are
// never overwritten.
func (c *Cache) Sync(refs []resume.Ref, opt SyncOptions) (Stats, error) {
	if opt.Translator == nil {
		opt.Translator = Mock{}
	}
	if opt.Delay == 0 {
		opt.Delay = 250 * time.Millisecond
	}

	seen := map[string]bool{}
	type job struct {
		ref resume.Ref
	}
	var todo []job
	for _, r := range refs {
		h := Hash(r.EN)
		if seen[h] {
			continue
		}
		seen[h] = true
		e, ok := c.get(r.EN)
		if ok && (e.DEAuto != "" || e.DEOverride != "") {
			continue
		}
		todo = append(todo, job{ref: r})
	}

	var errs []error
	for i, j := range todo {
		if i > 0 && !opt.DryRun {
			time.Sleep(opt.Delay)
		}
		de, err := opt.Translator.Translate(j.ref.EN)
		if err != nil {
			// One retry, then move on: a single flaky call must not abort a
			// long pass.
			time.Sleep(opt.Delay)
			de, err = opt.Translator.Translate(j.ref.EN)
		}
		if err == nil {
			de = opt.Glossary.Apply(de)
		}
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", j.ref.Path, err))
		case opt.DryRun:
			// Nothing is written, but the caller still sees what would happen.
		default:
			c.SetAuto(j.ref.EN, de, j.ref.Path)
			if opt.Save != nil {
				if saveErr := opt.Save(); saveErr != nil {
					errs = append(errs, saveErr)
				}
			}
		}
		if opt.Progress != nil {
			opt.Progress(i+1, len(todo), j.ref, de, err)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return c.StatsFor(refs), err
	}
	return c.StatsFor(refs), nil
}

// Unverified returns entries a human still has to look at, optionally
// filtered by a path prefix such as "work".
func (c *Cache) Unverified(prefix string) []Entry {
	out := make([]Entry, 0, len(c.Entries))
	for _, e := range c.Entries {
		if e.Verified || e.Orphaned {
			continue
		}
		if prefix != "" && !strings.HasPrefix(e.PathHint, prefix) {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PathHint < out[j].PathHint })
	return out
}

// EntriesView exposes a copy of the cache entries.
func (c *Cache) EntriesView() []Entry {
	out := make([]Entry, len(c.Entries))
	copy(out, c.Entries)
	return out
}

// Resolve is a lenient lookup used by the TUI: human edit, then machine
// translation, then English.
func (c *Cache) Resolve(en string) string {
	if v, ok := c.Lookup(en); ok {
		return v
	}
	return en
}
