// Package capsule stores one application as a self-contained folder.
//
// An application is not a CV variant in git. It is a capsule:
//
//	applications/2026-10-24-testgmbh/
//	  capsule.json                      the decisions that produced the PDFs
//	  rendered.json                     the exact CV that was compiled
//	  CV_Matt_Ji.pdf                    embedded capsule.json + rendered.json
//	  Cover_Letter_Matt_Ji.pdf          embedded capsule.json
//	  Application_Package_Matt_Ji.pdf   letter + CV + optional certificates
//
// Because the state travels inside the PDFs, an old application can be restored
// months later without a branch, a tag, or a stash.
package capsule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shaoyanji/jobby/internal/atomicfile"
	"github.com/shaoyanji/jobby/internal/coverletter"
	"github.com/shaoyanji/jobby/internal/loadout"
	"github.com/shaoyanji/jobby/internal/resume"
)

// BuildRecord is one export, kept for provenance.
type BuildRecord struct {
	At        string   `json:"at"`
	Lang      string   `json:"lang"`
	Pages     int      `json:"pages,omitempty"`
	Preset    string   `json:"preset,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}

// Capsule is capsule.json.
type Capsule struct {
	Version    int                `json:"version"`
	Company    string             `json:"company"`
	Role       string             `json:"role,omitempty"`
	Salutation string             `json:"salutation,omitempty"`
	Language   string             `json:"language"`
	Lens       string             `json:"lens"`
	Label      string             `json:"label,omitempty"`
	FileStem   string             `json:"file_stem,omitempty"`
	Titles     map[string]string  `json:"titles,omitempty"`
	Work       map[string][]int   `json:"work,omitempty"`
	Projects   map[string][]int   `json:"projects,omitempty"`
	Cover      coverletter.Letter `json:"cover_letter"`
	CreatedAt  string             `json:"created_at"`
	UpdatedAt  string             `json:"updated_at"`
	Builds     []BuildRecord      `json:"builds,omitempty"`
}

// New returns an empty capsule stamped with the creation time.
func New(company string) *Capsule {
	now := time.Now().UTC().Format(time.RFC3339)
	return &Capsule{
		Version:   1,
		Company:   strings.TrimSpace(company),
		Language:  "en",
		CreatedAt: now,
		UpdatedAt: now,
		Titles:    map[string]string{},
		Work:      map[string][]int{},
		Projects:  map[string][]int{},
		Cover:     coverletter.Letter{SourceLang: "en"},
	}
}

// Slug lowercases and keeps [a-z0-9], collapsing everything else to "-".
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-' || r == '_' || r == '.':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// DirName is the capsule folder name: <YYYY-MM-DD>-<company slug>.
func (c *Capsule) DirName() string {
	day := c.CreatedAt
	if t, err := time.Parse(time.RFC3339, c.CreatedAt); err == nil {
		day = t.UTC().Format("2006-01-02")
	} else if len(day) >= 10 {
		day = day[:10]
	} else {
		day = time.Now().UTC().Format("2006-01-02")
	}
	slug := Slug(c.Company)
	if slug == "" {
		slug = "application"
	}
	return day + "-" + slug
}

// Dir is the absolute capsule folder.
func (c *Capsule) Dir(root string) string {
	return filepath.Join(root, "applications", c.DirName())
}

// FilePath is <dir>/capsule.json.
func (c *Capsule) FilePath(root string) string {
	return filepath.Join(c.Dir(root), "capsule.json")
}

// Save writes capsule.json atomically and returns the folder.
func (c *Capsule) Save(root string) (string, error) {
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	dir := c.Dir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := atomicfile.WriteJSON(c.FilePath(root), c); err != nil {
		return "", err
	}
	return dir, nil
}

// Load reads a capsule from a folder or a capsule.json path.
func Load(path string) (*Capsule, error) {
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		path = filepath.Join(path, "capsule.json")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read capsule: %w", err)
	}
	var c Capsule
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse capsule %s: %w", path, err)
	}
	if c.Cover.SourceLang == "" {
		c.Cover.SourceLang = "en"
	}
	if c.Language == "" {
		c.Language = "en"
	}
	return &c, nil
}

// Find loads the newest capsule for a company, if any.
func Find(root, company string) (*Capsule, error) {
	sums, err := List(root)
	if err != nil {
		return nil, err
	}
	slug := Slug(company)
	for i := len(sums) - 1; i >= 0; i-- {
		if Slug(sums[i].Company) == slug {
			return Load(sums[i].Dir)
		}
	}
	return nil, nil
}

// Summary is one row in a capsule list.
type Summary struct {
	Dir       string
	Company   string
	Role      string
	Language  string
	Lens      string
	UpdatedAt string
}

// List reads every capsule under applications/, newest first.
func List(root string) ([]Summary, error) {
	base := filepath.Join(root, "applications")
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Summary
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(base, e.Name())
		c, err := Load(dir)
		if err != nil {
			continue // a folder without a capsule is not an application
		}
		out = append(out, Summary{
			Dir:       dir,
			Company:   c.Company,
			Role:      c.Role,
			Language:  c.Language,
			Lens:      c.Lens,
			UpdatedAt: c.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

// Apply writes the capsule's title overrides, persona and highlight selection
// into an export document. Selection is applied exactly once, so this must run
// on a document that still holds every bullet.
func (c *Capsule) Apply(doc *resume.Doc) {
	if c.Label != "" {
		if err := doc.SetString("basics.label", c.Label); err != nil {
			if basics, ok := doc.Root["basics"].(map[string]any); ok {
				basics["label"] = c.Label
			}
		}
	}
	for key, title := range c.Titles {
		if strings.TrimSpace(title) == "" {
			continue
		}
		for i, item := range doc.WorkItems() {
			if item.Key == key {
				_ = doc.SetString(fmt.Sprintf("work[%d].position", i), title)
			}
		}
	}
	loadout.Apply(doc, loadout.Loadout{Work: c.Work, Projects: c.Projects})
}

// Artifact names inside the capsule folder.
func (c *Capsule) Artifact(kind string) string {
	stem := c.FileStem
	if stem == "" {
		stem = "Resume"
	}
	switch kind {
	case "cv":
		return "CV_" + stem + ".pdf"
	case "letter":
		return "Cover_Letter_" + stem + ".pdf"
	case "package":
		return "Application_Package_" + stem + ".pdf"
	case "rendered":
		return "rendered.json"
	case "letterjson":
		return "letter.json"
	}
	return kind
}

// AddBuild records an export and trims the history to the last 20 builds.
func (c *Capsule) AddBuild(rec BuildRecord) {
	c.Builds = append(c.Builds, rec)
	if len(c.Builds) > 20 {
		c.Builds = c.Builds[len(c.Builds)-20:]
	}
}

// filenameStem turns a person's name into a file stem: "Shao-yan (Matt) Ji"
// becomes "Shao-yan_Ji" unless meta.file_stem says otherwise.
var nonName = regexp.MustCompile(`\([^)]*\)`)

// DefaultFileStem derives a stable stem from basics.name.
func DefaultFileStem(name string) string {
	name = nonName.ReplaceAllString(name, " ")
	fields := strings.Fields(name)
	if len(fields) > 2 {
		fields = fields[len(fields)-2:]
	}
	if len(fields) == 0 {
		return "Resume"
	}
	return strings.Join(fields, "_")
}
