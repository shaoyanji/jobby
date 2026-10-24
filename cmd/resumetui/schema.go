package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
)

// Ref locates a translatable English string inside the resume.
type Ref struct {
	Path string
	EN   string
}

type Location struct {
	City        string `json:"city,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	Region      string `json:"region,omitempty"`
}

type Profile struct {
	Network  string `json:"network,omitempty"`
	Username string `json:"username,omitempty"`
	URL      string `json:"url,omitempty"`
}

type Basics struct {
	Name      string    `json:"name,omitempty"`
	Label     string    `json:"label,omitempty"`
	AltLabels []string  `json:"altLabels,omitempty"`
	Image     string    `json:"image,omitempty"`
	Email     string    `json:"email,omitempty"`
	Phone     string    `json:"phone,omitempty"`
	URL       string    `json:"url,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Signature string    `json:"signature,omitempty"`
	Location  Location  `json:"location,omitempty"`
	Profiles  []Profile `json:"profiles,omitempty"`
}

type Work struct {
	Name       string   `json:"name,omitempty"`
	Position   string   `json:"position,omitempty"`
	URL        string   `json:"url,omitempty"`
	StartDate  string   `json:"startDate,omitempty"`
	EndDate    string   `json:"endDate,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Highlights []string `json:"highlights,omitempty"`
}

type Project struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	URL         string   `json:"url,omitempty"`
	Highlights  []string `json:"highlights,omitempty"`
}

type Skill struct {
	Name     string   `json:"name,omitempty"`
	Level    string   `json:"level,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
}

type Language struct {
	Language string `json:"language,omitempty"`
	Fluency  string `json:"fluency,omitempty"`
}

type Education struct {
	Institution string `json:"institution,omitempty"`
	Area        string `json:"area,omitempty"`
	StudyType   string `json:"studyType,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	EndDate     string `json:"endDate,omitempty"`
}

// Resume keeps every unknown key so custom fields (basics.signature,
// basics.altLabels, meta.rezi_source_id, meta.synced_tasks) survive a save.
type Resume struct {
	Basics    Basics         `json:"basics"`
	Work      []Work         `json:"work,omitempty"`
	Projects  []Project      `json:"projects,omitempty"`
	Skills    []Skill        `json:"skills,omitempty"`
	Languages []Language     `json:"languages,omitempty"`
	Education []Education    `json:"education,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`

	raw map[string]json.RawMessage
}

func (r *Resume) UnmarshalJSON(b []byte) error {
	type alias Resume
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*r = Resume(a)
	if r.Meta == nil {
		r.Meta = map[string]any{}
	}
	return json.Unmarshal(b, &r.raw)
}

func (r Resume) MarshalJSON() ([]byte, error) {
	out := make(map[string]json.RawMessage, len(r.raw)+8)
	for k, v := range r.raw {
		out[k] = v
	}
	set := func(key string, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		out[key] = b
		return nil
	}
	for key, v := range map[string]any{
		"basics":    r.Basics,
		"work":      r.Work,
		"projects":  r.Projects,
		"skills":    r.Skills,
		"languages": r.Languages,
		"education": r.Education,
		"meta":      r.Meta,
	} {
		if err := set(key, v); err != nil {
			return nil, err
		}
	}
	return json.Marshal(out)
}

func LoadResume(path string) (*Resume, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Resume
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *Resume) Save(path string) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// Translatable returns every EN string the pipeline may translate, in a stable
// order so cache diffs stay readable.
func (r *Resume) Translatable() []Ref {
	var refs []Ref
	add := func(path, s string) {
		if s != "" {
			refs = append(refs, Ref{Path: path, EN: s})
		}
	}
	add("basics.label", r.Basics.Label)
	add("basics.summary", r.Basics.Summary)
	for i, v := range r.Basics.AltLabels {
		add("basics.altLabels["+itoa(i)+"]", v)
	}
	for i, w := range r.Work {
		base := "work[" + itoa(i) + "]"
		add(base+".position", w.Position)
		add(base+".summary", w.Summary)
		for j, h := range w.Highlights {
			add(base+".highlights["+itoa(j)+"]", h)
		}
	}
	for i, p := range r.Projects {
		base := "projects[" + itoa(i) + "]"
		add(base+".description", p.Description)
		for j, h := range p.Highlights {
			add(base+".highlights["+itoa(j)+"]", h)
		}
	}
	for i, s := range r.Skills {
		add("skills["+itoa(i)+"].name", s.Name)
	}
	// Deduplicate repeated strings; the cache is keyed by content.
	seen := map[string]bool{}
	out := refs[:0]
	for _, ref := range refs {
		if seen[ref.EN] {
			continue
		}
		seen[ref.EN] = true
		out = append(out, ref)
	}
	return out
}

// ApplyLanguage rewrites translatable fields for the exported view. Missing
// translations fall back to English so a build never emits empty text.
func (r *Resume) ApplyLanguage(lang string, c Cache) (translated, missing int) {
	if lang == "" {
		lang = "en"
	}
	if r.Meta == nil {
		r.Meta = map[string]any{}
	}
	r.Meta["language"] = lang
	if lang == "en" {
		return 0, 0
	}
	tr := func(s string) string {
		if s == "" {
			return s
		}
		if v, ok := c.Lookup(s); ok {
			translated++
			return v
		}
		missing++
		return s
	}
	r.Basics.Label = tr(r.Basics.Label)
	r.Basics.Summary = tr(r.Basics.Summary)
	for i := range r.Basics.AltLabels {
		r.Basics.AltLabels[i] = tr(r.Basics.AltLabels[i])
	}
	for i := range r.Work {
		r.Work[i].Position = tr(r.Work[i].Position)
		r.Work[i].Summary = tr(r.Work[i].Summary)
		for j := range r.Work[i].Highlights {
			r.Work[i].Highlights[j] = tr(r.Work[i].Highlights[j])
		}
	}
	for i := range r.Projects {
		r.Projects[i].Description = tr(r.Projects[i].Description)
		for j := range r.Projects[i].Highlights {
			r.Projects[i].Highlights[j] = tr(r.Projects[i].Highlights[j])
		}
	}
	for i := range r.Skills {
		r.Skills[i].Name = tr(r.Skills[i].Name)
	}
	return translated, missing
}

func contentKey(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// sortedKeys is used by the TUI to keep list rendering deterministic.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
