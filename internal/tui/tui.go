// Package tui holds the optional interactive layer: a translation review deck
// and a loadout editor. Everything here has a headless equivalent on the CLI,
// so the state (i18n_cache.json, loadouts.json) is never UI-only.
package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/shaoyanji/jobby/internal/i18n"
	"github.com/shaoyanji/jobby/internal/loadout"
	"github.com/shaoyanji/jobby/internal/resume"
)

// Session configures the interactive forms.
type Session struct {
	In  io.Reader
	Out io.Writer
	Log func(format string, args ...any)
}

func (s Session) form(groups ...*huh.Group) *huh.Form {
	f := huh.NewForm(groups...)
	if s.In != nil {
		f = f.WithInput(s.In)
	}
	if s.Out != nil {
		f = f.WithOutput(s.Out)
	}
	return f
}

func (s Session) logf(format string, args ...any) {
	if s.Log != nil {
		s.Log(format, args...)
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Review walks every unverified translation and lets a human accept or rewrite
// it. Entries are saved one at a time, so quitting halfway keeps the work.
func Review(cache *i18n.Cache, pathPrefix string) error {
	return ReviewEntries(cache, cache.Unverified(pathPrefix))
}

// ReviewEntries is Review for an explicit entry set, which is how the workbench
// limits the deck to the strings the current application actually shows.
func ReviewEntries(cache *i18n.Cache, entries []i18n.Entry) error {
	if len(entries) == 0 {
		fmt.Println("No unverified translations.")
		return nil
	}
	fmt.Printf("Reviewing %d unverified translation(s). Ctrl+C leaves the rest for later.\n", len(entries))

	done := 0
	for _, entry := range entries {
		value := entry.DEOverride
		if value == "" {
			value = entry.DEAuto
		}
		verified := entry.Verified

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title(entry.PathHint).
					Description(clip(entry.EN, 400)),
				huh.NewText().
					Title("German").
					Value(&value).
					Lines(3).
					CharLimit(1200),
				huh.NewConfirm().
					Title("Mark verified?").
					Affirmative("Yes").
					Negative("Not yet").
					Value(&verified),
			),
		)
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Printf("\nStopped after %d review(s).\n", done)
				return nil
			}
			return err
		}
		cache.SetOverride(entry.EN, strings.TrimSpace(value), verified)
		if err := cache.Save(); err != nil {
			return err
		}
		done++
	}
	fmt.Printf("Reviewed %d translation(s).\n", done)
	return nil
}

// EditLoadouts builds a loadout with a guided deck: persona, then one
// multi-select per employer and project.
func EditLoadouts(doc *resume.Doc, file *loadout.File, name string) (loadout.Loadout, error) {
	if name == "" {
		name = file.Active
	}
	if name == "" {
		name = "default"
	}
	existing, _ := file.Get(name)
	if existing.Name == "" {
		existing = loadout.Full(doc, name)
	}

	persona := existing.Label
	if persona == "" {
		persona = doc.PrimaryLabel()
	}
	labels := doc.Labels()
	if len(labels) == 0 {
		labels = []string{persona}
	}

	work := doc.WorkItems()
	projects := doc.ProjectItems()
	workSel := make([][]int, len(work))
	projSel := make([][]int, len(projects))
	for i, item := range work {
		workSel[i] = preset(existing.Work[item.Key], len(item.Highlights))
	}
	for i, item := range projects {
		projSel[i] = preset(existing.Projects[item.Key], len(item.Highlights))
	}

	groups := []*huh.Group{huh.NewGroup(
		huh.NewInput().
			Title("Loadout name").
			Description("Written to loadouts.json; reusing a name updates it.").
			Value(&name),
		huh.NewSelect[string]().
			Title("Persona").
			Description("Shown under your name on the CV.").
			Options(huh.NewOptions(labels...)...).
			Value(&persona),
	)}

	for i, item := range work {
		title := item.Key
		if item.Detail != "" {
			title = fmt.Sprintf("%s — %s", item.Key, item.Detail)
		}
		groups = append(groups, huh.NewGroup(
			highlights(title, item.Highlights, &workSel[i]),
		))
	}
	for i, item := range projects {
		groups = append(groups, huh.NewGroup(
			highlights(item.Key, item.Highlights, &projSel[i]),
		))
	}

	stamp := true
	groups = append(groups, huh.NewGroup(
		huh.NewConfirm().
			Title("Stamp meta.synced_tasks?").
			Description("Records this deck and date in resume.json for provenance.").
			Value(&stamp),
	))

	if err := (&Session{}).form(groups...).Run(); err != nil {
		return loadout.Loadout{}, err
	}

	l := loadout.Loadout{
		Name:     strings.TrimSpace(name),
		Label:    persona,
		Language: doc.Language(),
		Work:     map[string][]int{},
		Projects: map[string][]int{},
	}
	for i, item := range work {
		l.Work[item.Key] = workSel[i]
	}
	for i, item := range projects {
		l.Projects[item.Key] = projSel[i]
	}
	if l.Name == "" {
		l.Name = "default"
	}
	file.Put(l)

	if stamp {
		tag := time.Now().UTC().Format("20060102") + "-" + l.Name
		tasks := doc.MetaStrings("synced_tasks")
		seen := false
		for _, t := range tasks {
			if t == tag {
				seen = true
				break
			}
		}
		if !seen {
			tasks = append(tasks, tag)
		}
		doc.SetMetaStrings("synced_tasks", tasks)
		doc.SetMeta("last_synced", tag)
	}
	return l, nil
}

func highlights(title string, bullets []string, sel *[]int) *huh.MultiSelect[int] {
	options := make([]huh.Option[int], 0, len(bullets))
	for i, b := range bullets {
		options = append(options, huh.NewOption(fmt.Sprintf("%d. %s", i, clip(b, 110)), i))
	}
	return huh.NewMultiSelect[int]().
		Title(title).
		Description("Space toggles, enter confirms. Deselected bullets leave the export, not resume.json.").
		Options(options...).
		Value(sel)
}

// preset keeps a saved selection, extends it when bullets were added, and
// falls back to "all" for a key that was never saved.
func preset(saved []int, n int) []int {
	if len(saved) == 0 {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, len(saved))
	for _, i := range saved {
		if i >= 0 && i < n {
			out = append(out, i)
		}
	}
	return out
}

// Action is one menu choice.
type Action string

const (
	ActionReview  Action = "review"
	ActionLoadout Action = "loadout"
	ActionSync    Action = "sync"
	ActionBuildEN Action = "build-en"
	ActionBuildDE Action = "build-de"
	ActionUnpack  Action = "unpack"
	ActionQuit    Action = "quit"
)

// Menu asks what to do next.
func Menu() (Action, error) {
	var choice string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("jobby").
			Options(
				huh.NewOption("Review German translations", string(ActionReview)),
				huh.NewOption("Edit a loadout", string(ActionLoadout)),
				huh.NewOption("Sync missing translations", string(ActionSync)),
				huh.NewOption("Build English PDF", string(ActionBuildEN)),
				huh.NewOption("Build German PDF", string(ActionBuildDE)),
				huh.NewOption("Quit", string(ActionQuit)),
			).
			Value(&choice),
	)).Run()
	if err != nil {
		return ActionQuit, err
	}
	return Action(choice), nil
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
