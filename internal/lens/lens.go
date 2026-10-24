// Package lens holds the narrative lenses: named ways of telling the same
// career. A lens shifts the persona line, rewrites per-company job titles, and
// pre-selects the bullets that belong to that story.
//
// A lens never edits the master resume. It describes a view, which is why three
// applications can exist at once without three copies of resume.json.
package lens

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/shaoyanji/jobby/internal/resume"
)

// Lens is one narrative.
type Lens struct {
	// ID is the key in lenses.json; it is not part of the object itself.
	ID string `json:"-"`
	// Name is the human label shown in menus.
	Name string `json:"name"`
	// Label replaces basics.label.
	Label string `json:"label,omitempty"`
	// Summary replaces basics.summary when set.
	Summary string `json:"summary,omitempty"`
	// Titles overrides work[].position, keyed by the same company key a
	// loadout uses.
	Titles map[string]string `json:"titles,omitempty"`
	// Keywords prioritise bullets. An item with no explicit selection keeps
	// the bullets that mention any keyword (word-bounded), capped at
	// MaxHighlights; an item with no match keeps everything.
	Keywords []string `json:"keywords,omitempty"`
	// MaxHighlights caps keyword matches per item. Zero means 3.
	MaxHighlights int `json:"max_highlights,omitempty"`
	// Work and Projects are explicit selections and win over keywords.
	Work map[string][]int `json:"work,omitempty"`
	// Projects does the same for project highlights.
	Projects map[string][]int `json:"projects,omitempty"`
}

// File is lenses.json.
type File struct {
	Version int             `json:"version"`
	Active  string          `json:"active"`
	Lenses  map[string]Lens `json:"lenses"`
}

// Load reads lenses.json. A missing file is an empty file, not an error.
func Load(path string) (*File, error) {
	f := &File{Version: 1, Lenses: map[string]Lens{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read lenses: %w", err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse lenses %s: %w", path, err)
	}
	if f.Lenses == nil {
		f.Lenses = map[string]Lens{}
	}
	for id, l := range f.Lenses {
		l.ID = id
		f.Lenses[id] = l
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return f, nil
}

// Get resolves a lens by id. An empty id falls back to the file's active lens,
// then to the first lens by name, so `build` works with no flags.
func (f *File) Get(id string) (Lens, bool) {
	if id == "" {
		id = f.Active
	}
	if l, ok := f.Lenses[id]; ok {
		return l, true
	}
	if id != "" {
		return Lens{}, false
	}
	for _, name := range f.IDs() {
		return f.Lenses[name], true
	}
	return Lens{}, false
}

// IDs lists lens ids alphabetically.
func (f *File) IDs() []string {
	out := make([]string, 0, len(f.Lenses))
	for id := range f.Lenses {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Options renders the lenses as "name (id)" for a select field.
func (f *File) Options() []string {
	out := make([]string, 0, len(f.Lenses))
	for _, id := range f.IDs() {
		out = append(out, fmt.Sprintf("%s (%s)", f.Lenses[id].Name, id))
	}
	return out
}

// Identity writes the lens persona and title overrides into a document. It does
// not touch highlights: selection is applied once, from the capsule.
func Identity(doc *resume.Doc, l Lens) {
	if l.Label != "" {
		if err := doc.SetString("basics.label", l.Label); err != nil {
			if basics, ok := doc.Root["basics"].(map[string]any); ok {
				basics["label"] = l.Label
			}
		}
	}
	if l.Summary != "" {
		_ = doc.SetString("basics.summary", l.Summary)
	}
	for key, title := range l.Titles {
		for i, item := range doc.WorkItems() {
			if item.Key == key {
				_ = doc.SetString(fmt.Sprintf("work[%d].position", i), title)
			}
		}
	}
}

// Selection computes the highlight selection this lens implies, without
// mutating the document. Explicit entries win; otherwise keyword matching picks
// up to MaxHighlights bullets; an item with no keyword hit keeps every bullet.
func (l Lens) Selection(doc *resume.Doc) (work, projects map[string][]int) {
	re := l.keywordPattern()
	limit := l.MaxHighlights
	if limit <= 0 {
		limit = 3
	}
	pick := func(items []resume.Item, explicit map[string][]int) map[string][]int {
		out := make(map[string][]int, len(items))
		for _, item := range items {
			if idx, ok := explicit[item.Key]; ok {
				out[item.Key] = idx
				continue
			}
			if re == nil {
				out[item.Key] = all(len(item.Highlights))
				continue
			}
			var hits []int
			for i, bullet := range item.Highlights {
				if re.MatchString(bullet) {
					hits = append(hits, i)
				}
			}
			switch {
			case len(hits) == 0:
				out[item.Key] = all(len(item.Highlights))
			case len(hits) > limit:
				out[item.Key] = hits[:limit]
			default:
				out[item.Key] = hits
			}
		}
		return out
	}
	return pick(doc.WorkItems(), l.Work), pick(doc.ProjectItems(), l.Projects)
}

func (l Lens) keywordPattern() *regexp.Regexp {
	words := make([]string, 0, len(l.Keywords))
	for _, k := range l.Keywords {
		k = strings.TrimSpace(k)
		if k != "" {
			words = append(words, regexp.QuoteMeta(k))
		}
	}
	if len(words) == 0 {
		return nil
	}
	re, err := regexp.Compile(`(?i)\b(?:` + strings.Join(words, "|") + `)\b`)
	if err != nil {
		return nil
	}
	return re
}

func all(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}
