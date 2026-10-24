package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	SourceEN  string `json:"source_en"`
	DEAuto    string `json:"de_auto"`
	DEHuman   string `json:"de_human"`
	Status    string `json:"status"` // untranslated | auto | verified
	UpdatedAt string `json:"updated_at"`
}

// Cache is i18n_cache.json itself: sha256(english) -> Entry.
type Cache map[string]Entry

func LoadCache(path string) (Cache, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Cache{}, nil
	}
	if err != nil {
		return nil, err
	}
	c := Cache{}
	if len(strings.TrimSpace(string(b))) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func (c Cache) Save(path string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Lookup prefers a human edit, then the machine translation.
func (c Cache) Lookup(en string) (string, bool) {
	e, ok := c[contentKey(en)]
	if !ok {
		return "", false
	}
	if e.DEHuman != "" {
		return e.DEHuman, true
	}
	if e.DEAuto != "" {
		return e.DEAuto, true
	}
	return "", false
}

func (c Cache) Status(en string) string {
	e, ok := c[contentKey(en)]
	switch {
	case !ok || (e.DEAuto == "" && e.DEHuman == ""):
		return "missing"
	case e.DEHuman != "":
		return "verified"
	default:
		return "auto"
	}
}

// Sync translates every string with no machine translation yet, two at a time,
// and never overwrites a human edit.
func (c Cache) Sync(refs []Ref, progress func(done, total int, s string)) (int, []error) {
	var todo []Ref
	for _, r := range refs {
		e, ok := c[contentKey(r.EN)]
		if ok && (e.DEAuto != "" || e.DEHuman != "") {
			continue
		}
		todo = append(todo, r)
	}
	if len(todo) == 0 {
		return 0, nil
	}
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	done := 0
	for _, ref := range todo {
		wg.Add(1)
		go func(ref Ref) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			de, err := translate(ref.EN)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", ref.Path, err))
			} else {
				key := contentKey(ref.EN)
				e := c[key]
				e.SourceEN = ref.EN
				e.DEAuto = de
				e.Status = "auto"
				e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				c[key] = e
			}
			done++
			if progress != nil {
				progress(done, len(todo), ref.Path)
			}
		}(ref)
	}
	wg.Wait()
	return len(todo), errs
}

func translate(en string) (string, error) {
	cmd := exec.Command("trans", "-b", "en:de", en)
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return "", fmt.Errorf("trans: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	de := strings.TrimSpace(string(out))
	if de == "" {
		return "", fmt.Errorf("trans returned nothing")
	}
	return de, nil
}

func (c Cache) SetHuman(en, de string) {
	key := contentKey(en)
	e, ok := c[key]
	if !ok {
		e = Entry{SourceEN: en}
	}
	e.SourceEN = en
	e.DEHuman = de
	if de == "" {
		e.Status = "auto"
	} else {
		e.Status = "verified"
	}
	e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	c[key] = e
}

func (c Cache) Coverage(refs []Ref) (verified, auto, missing int) {
	for _, r := range refs {
		switch c.Status(r.EN) {
		case "verified":
			verified++
		case "auto":
			auto++
		default:
			missing++
		}
	}
	return
}
