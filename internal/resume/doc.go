// Package resume reads and writes JSON Resume documents.
//
// The document is kept as a generic JSON tree, not a struct, so keys this
// package does not know about (basics.signature, basics.altLabels,
// meta.rezi_source_id, meta.synced_tasks) survive every read/write cycle.
package resume

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Doc is a parsed resume plus the path it came from.
type Doc struct {
	Root map[string]any
	Path string
}

// Load parses a JSON Resume file. Numbers are kept as json.Number so an
// unrelated export cannot turn 2 into 2.0000000000000004.
func Load(path string) (*Doc, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read resume: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("parse resume %s: %w", path, err)
	}
	if len(root) == 0 {
		return nil, fmt.Errorf("parse resume %s: empty object", path)
	}
	return &Doc{Root: root, Path: path}, nil
}

// Save writes the document back with two-space indentation and a trailing
// newline, so diffs stay reviewable.
func (d *Doc) Save(path string) error {
	b, err := json.MarshalIndent(d.Root, "", "  ")
	if err != nil {
		return fmt.Errorf("encode resume: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write resume: %w", err)
	}
	return nil
}

// Clone returns a deep copy, used by exports that must not mutate the source.
func (d *Doc) Clone() *Doc {
	b, err := json.Marshal(d.Root)
	if err != nil {
		panic(err) // the tree was decoded from JSON, so it always re-encodes
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		panic(err)
	}
	return &Doc{Root: root, Path: d.Path}
}

type step struct {
	key     string
	index   int
	indexed bool
}

// parsePath splits "work[0].highlights[2]" into steps.
func parsePath(p string) ([]step, error) {
	var steps []step
	for i := 0; i < len(p); {
		switch p[i] {
		case '.':
			i++
		case '[':
			end := strings.IndexByte(p[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("path %q: unterminated index", p)
			}
			n, err := strconv.Atoi(p[i+1 : i+end])
			if err != nil {
				return nil, fmt.Errorf("path %q: bad index: %w", p, err)
			}
			if len(steps) == 0 || steps[len(steps)-1].indexed {
				return nil, fmt.Errorf("path %q: index without a key", p)
			}
			steps[len(steps)-1].indexed = true
			steps[len(steps)-1].index = n
			i += end + 1
		default:
			j := i
			for j < len(p) && p[j] != '.' && p[j] != '[' {
				j++
			}
			steps = append(steps, step{key: p[i:j]})
			i = j
		}
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("path %q: empty", p)
	}
	return steps, nil
}

// lookup walks a path. A step names a key and may then index the array that
// key holds, which is what "work[0].highlights[1]" needs.
func lookup(cur any, steps []step) (any, error) {
	for _, s := range steps {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object at %q", s.key)
		}
		v, ok := m[s.key]
		if !ok {
			return nil, fmt.Errorf("missing key %q", s.key)
		}
		if !s.indexed {
			cur = v
			continue
		}
		arr, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("expected array at %q", s.key)
		}
		if s.index < 0 || s.index >= len(arr) {
			return nil, fmt.Errorf("index %d out of range at %q", s.index, s.key)
		}
		cur = arr[s.index]
	}
	return cur, nil
}

func setIn(cur any, steps []step, v any) (any, error) {
	if len(steps) == 0 {
		return v, nil
	}
	s := steps[0]
	m, ok := cur.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object at %q", s.key)
	}
	existing, ok := m[s.key]
	if !ok {
		// Refuse to invent a key: a typo in a path hint must fail loudly.
		return nil, fmt.Errorf("missing key %q", s.key)
	}
	if !s.indexed {
		next, err := setIn(existing, steps[1:], v)
		if err != nil {
			return nil, err
		}
		m[s.key] = next
		return m, nil
	}
	arr, ok := existing.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array at %q", s.key)
	}
	if s.index < 0 || s.index >= len(arr) {
		return nil, fmt.Errorf("index %d out of range at %q", s.index, s.key)
	}
	next, err := setIn(arr[s.index], steps[1:], v)
	if err != nil {
		return nil, err
	}
	arr[s.index] = next
	m[s.key] = arr
	return m, nil
}

// Get returns the value at a path, if it exists.
func (d *Doc) Get(path string) (any, bool) {
	steps, err := parsePath(path)
	if err != nil {
		return nil, false
	}
	v, err := lookup(d.Root, steps)
	return v, err == nil
}

// GetString returns the string at a path, with ok=false when it is absent,
// null, or not a string.
func (d *Doc) GetString(path string) (string, bool) {
	v, ok := d.Get(path)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// SetString writes a string at a path that already exists. It refuses to
// create keys: a typo in a path hint should fail loudly, not invent a field.
func (d *Doc) SetString(path, value string) error {
	steps, err := parsePath(path)
	if err != nil {
		return err
	}
	root, err := setIn(d.Root, steps, value)
	if err != nil {
		return fmt.Errorf("set %s: %w", path, err)
	}
	d.Root = root.(map[string]any)
	return nil
}

// Set writes any value at a path that already exists.
func (d *Doc) Set(path string, value any) error {
	steps, err := parsePath(path)
	if err != nil {
		return err
	}
	root, err := setIn(d.Root, steps, value)
	if err != nil {
		return fmt.Errorf("set %s: %w", path, err)
	}
	d.Root = root.(map[string]any)
	return nil
}

// Strings returns the string slice at a path; an absent or non-array value
// yields nil.
func (d *Doc) Strings(path string) []string {
	v, ok := d.Get(path)
	if !ok {
		return nil
	}
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

// Objects returns the object slice at a path.
func (d *Doc) Objects(path string) []map[string]any {
	v, ok := d.Get(path)
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// Len reports the number of elements at an array path.
func (d *Doc) Len(path string) int {
	v, ok := d.Get(path)
	if !ok {
		return 0
	}
	if arr, ok := v.([]any); ok {
		return len(arr)
	}
	return 0
}

// Meta returns the meta object, creating it when absent.
func (d *Doc) Meta() map[string]any {
	m, ok := d.Root["meta"].(map[string]any)
	if !ok {
		m = map[string]any{}
		d.Root["meta"] = m
	}
	return m
}

// SetMeta writes one meta key.
func (d *Doc) SetMeta(key string, value any) {
	d.Meta()[key] = value
}

// MetaString reads one meta key as a string.
func (d *Doc) MetaString(key string) string {
	if s, ok := d.Meta()[key].(string); ok {
		return s
	}
	return ""
}

// MetaStrings reads one meta key as a string slice.
func (d *Doc) MetaStrings(key string) []string {
	arr, ok := d.Meta()[key].([]any)
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

// SetMetaStrings writes one meta key as a string slice.
func (d *Doc) SetMetaStrings(key string, values []string) {
	arr := make([]any, 0, len(values))
	for _, v := range values {
		arr = append(arr, v)
	}
	d.Meta()[key] = arr
}

// ErrProfile is returned when a loadout names a profile that does not exist.
var ErrProfile = errors.New("unknown loadout profile")
