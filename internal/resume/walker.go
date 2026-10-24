package resume

import (
	"fmt"
	"strings"
)

// Ref is one translatable string: where it lives and what it says.
type Ref struct {
	Path string
	EN   string
}

// Refs walks every translatable field in document order. Duplicates are kept
// here (the same sentence in two bullets is two placements); callers that key
// by content dedupe for themselves.
func (d *Doc) Refs() []Ref {
	var refs []Ref
	add := func(path, s string) {
		if strings.TrimSpace(s) == "" {
			return
		}
		refs = append(refs, Ref{Path: path, EN: s})
	}

	add("basics.label", d.str("basics.label"))
	add("basics.summary", d.str("basics.summary"))
	for i, v := range d.Strings("basics.altLabels") {
		add(fmt.Sprintf("basics.altLabels[%d]", i), v)
	}

	for i := range d.Objects("work") {
		base := fmt.Sprintf("work[%d]", i)
		add(base+".position", d.str(base+".position"))
		add(base+".description", d.str(base+".description"))
		add(base+".summary", d.str(base+".summary"))
		for j, h := range d.Strings(base + ".highlights") {
			add(fmt.Sprintf("%s.highlights[%d]", base, j), h)
		}
	}

	for i := range d.Objects("projects") {
		base := fmt.Sprintf("projects[%d]", i)
		add(base+".description", d.str(base+".description"))
		for j, h := range d.Strings(base + ".highlights") {
			add(fmt.Sprintf("%s.highlights[%d]", base, j), h)
		}
		for j, r := range d.Strings(base + ".roles") {
			add(fmt.Sprintf("%s.roles[%d]", base, j), r)
		}
	}

	// Skill names are visible prose ("Distributed Systems"); keywords are
	// product names and stay untouched by design.
	for i := range d.Objects("skills") {
		add(fmt.Sprintf("skills[%d].name", i), d.str(fmt.Sprintf("skills[%d].name", i)))
	}

	for i := range d.Objects("languages") {
		base := fmt.Sprintf("languages[%d]", i)
		add(base+".language", d.str(base+".language"))
		add(base+".fluency", d.str(base+".fluency"))
	}

	for i := range d.Objects("education") {
		base := fmt.Sprintf("education[%d]", i)
		add(base+".area", d.str(base+".area"))
		add(base+".studyType", d.str(base+".studyType"))
		for j, c := range d.Strings(base + ".courses") {
			add(fmt.Sprintf("%s.courses[%d]", base, j), c)
		}
	}

	return refs
}

// Translatable returns refs deduplicated by content, keeping the first path.
func (d *Doc) Translatable() []Ref {
	seen := map[string]bool{}
	out := make([]Ref, 0, 32)
	for _, r := range d.Refs() {
		if seen[r.EN] {
			continue
		}
		seen[r.EN] = true
		out = append(out, r)
	}
	return out
}

func (d *Doc) str(path string) string {
	s, _ := d.GetString(path)
	return s
}

// Item is one work entry or project, flattened for selection UIs.
type Item struct {
	// Key is the stable identifier used in loadouts.json: the company or
	// project name, falling back to the array index when unnamed.
	Key        string
	Name       string
	Detail     string
	Highlights []string
}

// WorkItems returns every work entry with its highlight list.
func (d *Doc) WorkItems() []Item {
	return d.items("work", "position")
}

// ProjectItems returns every project with its highlight list.
func (d *Doc) ProjectItems() []Item {
	return d.items("projects", "")
}

func (d *Doc) items(collection, detailKey string) []Item {
	objs := d.Objects(collection)
	keys := d.Keys(collection)
	out := make([]Item, 0, len(objs))
	for i, obj := range objs {
		name, _ := obj["name"].(string)
		detail := ""
		if detailKey != "" {
			detail, _ = obj[detailKey].(string)
		}
		out = append(out, Item{
			Key:        keys[i],
			Name:       name,
			Detail:     detail,
			Highlights: d.Strings(fmt.Sprintf("%s[%d].highlights", collection, i)),
		})
	}
	return out
}

// Keys returns the stable loadout keys of a collection: the company or project
// name, falling back to "#<index>" for unnamed entries and disambiguating a
// repeated name by position.
func (d *Doc) Keys(collection string) []string {
	objs := d.Objects(collection)
	out := make([]string, 0, len(objs))
	used := map[string]int{}
	for i, obj := range objs {
		name, _ := obj["name"].(string)
		key := strings.TrimSpace(name)
		if key == "" {
			key = fmt.Sprintf("#%d", i)
		}
		if n := used[key]; n > 0 {
			key = fmt.Sprintf("%s (%d)", key, n+1)
		}
		used[key]++
		out = append(out, key)
	}
	return out
}

// Labels lists the primary label followed by every alternative persona.
func (d *Doc) Labels() []string {
	var out []string
	if l := d.str("basics.label"); l != "" {
		out = append(out, l)
	}
	for _, l := range d.Strings("basics.altLabels") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// PrimaryLabel is basics.label, used as the default persona.
func (d *Doc) PrimaryLabel() string { return d.str("basics.label") }

// Name is basics.name.
func (d *Doc) Name() string { return d.str("basics.name") }

// Language is meta.language, defaulting to "en".
func (d *Doc) Language() string {
	if l := d.MetaString("language"); l != "" {
		return l
	}
	return "en"
}

// SetLanguage stamps meta.language.
func (d *Doc) SetLanguage(lang string) { d.SetMeta("language", lang) }
