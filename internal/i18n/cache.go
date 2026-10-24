// Package i18n keeps German translations in a content-addressed sidecar.
//
// i18n_cache.json is a flat JSON array so it opens as a table in VisiData
// (`vd i18n_cache.json`) or reads cleanly from jq. Each entry is keyed by a
// 12-char SHA-256 of the trimmed English string, so editing an English bullet
// produces a cache miss instead of a silently stale German translation.
package i18n

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shaoyanji/jobby/internal/resume"
)

// Entry is one cached translation.
type Entry struct {
	Hash       string `json:"hash"`
	PathHint   string `json:"path_hint"`
	EN         string `json:"en"`
	DEAuto     string `json:"de_auto"`
	DEOverride string `json:"de_override"`
	Verified   bool   `json:"verified"`
	UpdatedAt  string `json:"updated_at"`
	// Orphaned marks an entry whose English string no longer appears in
	// resume.json. It is kept, not deleted, so an undone edit is a cache hit.
	Orphaned bool `json:"orphaned,omitempty"`
}

// Cache is the sidecar plus a hash index.
type Cache struct {
	Entries []Entry        `json:"-"`
	index   map[string]int // hash -> position in Entries
	path    string
}

// Hash is the cache key: sha256 of the trimmed English, first 12 hex chars.
func Hash(s string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(sum[:6])
}

// Load reads the cache. A missing file is an empty cache, not an error.
//
// The previous layout was an object keyed by full sha256
// ({"<hash>": {"source_en": ..., "de_human": ...}}); that shape is migrated on
// read so an existing file keeps its human translations.
func Load(path string) (*Cache, error) {
	c := &Cache{index: map[string]int{}, path: path}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read i18n cache: %w", err)
	}
	if strings.TrimSpace(string(b)) == "" {
		return c, nil
	}
	trimmed := strings.TrimSpace(string(b))
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(b, &c.Entries); err != nil {
			return nil, fmt.Errorf("parse i18n cache %s: %w", path, err)
		}
	} else {
		if err := c.migrateLegacy(b); err != nil {
			return nil, fmt.Errorf("parse legacy i18n cache %s: %w", path, err)
		}
	}
	for i := range c.Entries {
		e := &c.Entries[i]
		if e.Hash == "" {
			e.Hash = Hash(e.EN)
		}
		c.index[e.Hash] = i
	}
	return c, nil
}

func (c *Cache) migrateLegacy(b []byte) error {
	var legacy map[string]struct {
		SourceEN   string `json:"source_en"`
		DEAuto     string `json:"de_auto"`
		DEHuman    string `json:"de_human"`
		DEOverride string `json:"de_override"`
		Status     string `json:"status"`
		UpdatedAt  string `json:"updated_at"`
	}
	if err := json.Unmarshal(b, &legacy); err != nil {
		return err
	}
	keys := make([]string, 0, len(legacy))
	for k := range legacy {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := legacy[k]
		human := v.DEHuman
		if human == "" {
			human = v.DEOverride
		}
		c.Entries = append(c.Entries, Entry{
			Hash:       Hash(v.SourceEN),
			EN:         v.SourceEN,
			DEAuto:     v.DEAuto,
			DEOverride: human,
			Verified:   v.Status == "verified" || human != "",
			UpdatedAt:  v.UpdatedAt,
		})
	}
	return nil
}

// Path is the file this cache was loaded from.
func (c *Cache) Path() string { return c.path }

// Save writes the flat array atomically, so an interrupted sync cannot leave a
// truncated cache behind.
func (c *Cache) Save() error {
	return c.SaveAs(c.path)
}

// SaveAs writes the cache to an explicit path.
func (c *Cache) SaveAs(path string) error {
	if path == "" {
		return nil
	}
	sort.SliceStable(c.Entries, func(i, j int) bool {
		a, b := c.Entries[i], c.Entries[j]
		if a.Orphaned != b.Orphaned {
			return !a.Orphaned
		}
		return a.Hash < b.Hash
	})
	c.reindex()
	b, err := json.MarshalIndent(c.Entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode i18n cache: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".i18n-cache-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (c *Cache) reindex() {
	c.index = make(map[string]int, len(c.Entries))
	for i := range c.Entries {
		c.index[c.Entries[i].Hash] = i
	}
}

// get returns the entry for an English string.
func (c *Cache) get(en string) (*Entry, bool) {
	i, ok := c.index[Hash(en)]
	if !ok {
		return nil, false
	}
	return &c.Entries[i], true
}

// Status describes how well one string is translated.
type Status string

const (
	StatusMissing  Status = "missing"
	StatusAuto     Status = "auto"
	StatusVerified Status = "verified"
)

// StatusOf reports the translation state of one English string.
func (c *Cache) StatusOf(en string) Status {
	e, ok := c.get(en)
	switch {
	case !ok, e.DEAuto == "" && e.DEOverride == "":
		return StatusMissing
	case e.Verified || e.DEOverride != "":
		return StatusVerified
	default:
		return StatusAuto
	}
}

// Lookup resolves one English string. A human override always wins; otherwise
// the machine translation is used.
func (c *Cache) Lookup(en string) (string, bool) {
	e, ok := c.get(en)
	if !ok {
		return "", false
	}
	if e.DEOverride != "" {
		return e.DEOverride, true
	}
	if e.DEAuto != "" {
		return e.DEAuto, true
	}
	return "", false
}

// SetAuto stores a machine translation, preserving any human override.
func (c *Cache) SetAuto(en, de, pathHint string) {
	c.upsert(resume.Ref{Path: pathHint, EN: en}, func(e *Entry) {
		e.DEAuto = de
		e.Orphaned = false
	})
}

// SetOverride stores a human translation and its verified flag.
func (c *Cache) SetOverride(en, de string, verified bool) {
	e, ok := c.get(en)
	if !ok {
		c.Entries = append(c.Entries, Entry{Hash: Hash(en), EN: en})
		e = &c.Entries[len(c.Entries)-1]
	}
	e.Hash = Hash(en)
	e.DEOverride = de
	e.Verified = verified
	if en != "" {
		e.EN = en
	}
	e.Orphaned = false
	e.UpdatedAt = now()
	c.reindex()
}

func (c *Cache) upsert(ref resume.Ref, mutate func(*Entry)) {
	e, ok := c.get(ref.EN)
	if !ok {
		c.Entries = append(c.Entries, Entry{
			Hash:      Hash(ref.EN),
			PathHint:  ref.Path,
			EN:        ref.EN,
			UpdatedAt: now(),
		})
		e = &c.Entries[len(c.Entries)-1]
	}
	if e.EN == "" {
		e.EN = ref.EN
	}
	if e.PathHint == "" {
		e.PathHint = ref.Path
	}
	mutate(e)
	e.UpdatedAt = now()
	c.reindex()
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// Stats summarises translation coverage for one document.
type Stats struct {
	Total    int `json:"total"`
	Verified int `json:"verified"`
	Auto     int `json:"auto"`
	Missing  int `json:"missing"`
	Orphaned int `json:"orphaned"`
}

// String renders the summary the CLI prints after a sync.
func (s Stats) String() string {
	return fmt.Sprintf("%d strings · %d verified · %d auto · %d missing · %d orphaned",
		s.Total, s.Verified, s.Auto, s.Missing, s.Orphaned)
}

// StatsFor counts coverage over the deduplicated refs of a document.
func (c *Cache) StatsFor(refs []resume.Ref) Stats {
	seen := map[string]bool{}
	s := Stats{}
	for _, r := range refs {
		if seen[r.EN] {
			continue
		}
		seen[r.EN] = true
		s.Total++
		switch c.StatusOf(r.EN) {
		case StatusVerified:
			s.Verified++
		case StatusAuto:
			s.Auto++
		default:
			s.Missing++
		}
	}
	for _, e := range c.Entries {
		if e.Orphaned {
			s.Orphaned++
		}
	}
	return s
}

// MarkOrphans flags entries whose English no longer exists in the document and
// clears the flag on entries that came back.
func (c *Cache) MarkOrphans(refs []resume.Ref) int {
	live := make(map[string]bool, len(refs))
	for _, r := range refs {
		live[Hash(r.EN)] = true
	}
	newly := 0
	for i := range c.Entries {
		e := &c.Entries[i]
		shouldOrphan := !live[e.Hash]
		if shouldOrphan && !e.Orphaned {
			newly++
		}
		e.Orphaned = shouldOrphan
	}
	return newly
}

// Apply translates a document in place and stamps meta.language. Missing
// translations fall back to English so a build never emits empty text.
func Apply(doc *resume.Doc, c *Cache, lang string) (translated, missing int) {
	if lang == "" {
		lang = "en"
	}
	doc.SetLanguage(lang)
	if lang == "en" {
		return 0, 0
	}
	for _, ref := range doc.Refs() {
		de, ok := c.Lookup(ref.EN)
		if !ok || de == "" {
			missing++
			continue
		}
		if err := doc.SetString(ref.Path, de); err != nil {
			missing++
			continue
		}
		translated++
	}
	return translated, missing
}
