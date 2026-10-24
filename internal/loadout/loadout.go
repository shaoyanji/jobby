// Package loadout stores named decks: a persona plus the subset of highlight
// bullets that should survive an export.
//
// Loadouts are non-destructive. They never edit resume.json; they describe a
// view of it, so switching from "energy risk" to "AI systems" is one flag.
package loadout

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/shaoyanji/jobby/internal/resume"
)

// Loadout is a saved deck.
type Loadout struct {
	Name string `json:"name"`
	// Label is the persona shown under the name; must be basics.label or one
	// of basics.altLabels in the source resume.
	Label string `json:"label,omitempty"`
	// Work maps a work key (company name) to the highlight indices to keep,
	// in the order to keep them. A missing key means "keep everything".
	Work map[string][]int `json:"work,omitempty"`
	// Projects does the same for project highlights.
	Projects map[string][]int `json:"projects,omitempty"`
	Language string           `json:"language,omitempty"`
}

// File is loadouts.json.
type File struct {
	Version  int                `json:"version"`
	Active   string             `json:"active"`
	Loadouts map[string]Loadout `json:"loadouts"`
}

// Load reads loadouts.json. A missing file yields an empty file.
func Load(path string) (*File, error) {
	f := &File{Version: 1, Loadouts: map[string]Loadout{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read loadouts: %w", err)
	}
	if len(b) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse loadouts %s: %w", path, err)
	}
	if f.Loadouts == nil {
		f.Loadouts = map[string]Loadout{}
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return f, nil
}

// Save writes loadouts.json with a trailing newline.
func (f *File) Save(path string) error {
	if f.Loadouts == nil {
		f.Loadouts = map[string]Loadout{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode loadouts: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write loadouts: %w", err)
	}
	return nil
}

// Names lists profiles alphabetically.
func (f *File) Names() []string {
	out := make([]string, 0, len(f.Loadouts))
	for n := range f.Loadouts {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Get fetches one profile.
func (f *File) Get(name string) (Loadout, bool) {
	l, ok := f.Loadouts[name]
	return l, ok
}

// Put stores a profile and makes it active.
func (f *File) Put(l Loadout) {
	if f.Loadouts == nil {
		f.Loadouts = map[string]Loadout{}
	}
	f.Loadouts[l.Name] = l
	f.Active = l.Name
}

// EnsureDefault returns the named profile, creating a full-selection loadout
// the first time a name is used. This is what makes `--profile default` work on
// a fresh clone.
func (f *File) EnsureDefault(doc *resume.Doc, name string) Loadout {
	if l, ok := f.Get(name); ok {
		return l
	}
	l := Full(doc, name)
	f.Put(l)
	return l
}

// Full builds a loadout with every highlight selected and the primary persona.
func Full(doc *resume.Doc, name string) Loadout {
	l := Loadout{
		Name:     name,
		Label:    doc.PrimaryLabel(),
		Work:     map[string][]int{},
		Projects: map[string][]int{},
		Language: doc.Language(),
	}
	for _, item := range doc.WorkItems() {
		l.Work[item.Key] = indices(len(item.Highlights))
	}
	for _, item := range doc.ProjectItems() {
		l.Projects[item.Key] = indices(len(item.Highlights))
	}
	return l
}

func indices(n int) []int {
	if n == 0 {
		return []int{}
	}
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// Apply rewrites an exported copy: persona, then only the selected highlights.
// A work or project key that is absent from the loadout keeps all its bullets,
// so a new role is never silently dropped from the CV.
func Apply(doc *resume.Doc, l Loadout) {
	if l.Label != "" {
		if err := doc.SetString("basics.label", l.Label); err != nil {
			if basics, ok := doc.Root["basics"].(map[string]any); ok {
				basics["label"] = l.Label
			}
		}
	}
	setHighlights(doc, "work", l.Work)
	setHighlights(doc, "projects", l.Projects)
}

func setHighlights(doc *resume.Doc, collection string, selection map[string][]int) {
	if len(selection) == 0 {
		return
	}
	arr, ok := doc.Root[collection].([]any)
	if !ok {
		return
	}
	keys := doc.Keys(collection)
	for i := range arr {
		obj, ok := arr[i].(map[string]any)
		if !ok {
			continue
		}
		idx, ok := selection[keys[i]]
		if !ok {
			continue // not part of this deck: keep every bullet
		}
		all := stringSlice(obj["highlights"])
		if len(all) == 0 {
			continue // nothing to trim, and no empty array to invent
		}
		picked := make([]any, 0, len(idx))
		for _, n := range idx {
			if n >= 0 && n < len(all) {
				picked = append(picked, all[n])
			}
		}
		obj["highlights"] = picked
	}
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
