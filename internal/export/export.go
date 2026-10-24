// Package export materializes the view a PDF is built from: the source resume
// after a loadout has trimmed its bullets and a language pass has replaced its
// prose.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shaoyanji/jobby/internal/i18n"
	"github.com/shaoyanji/jobby/internal/loadout"
	"github.com/shaoyanji/jobby/internal/resume"
)

// Request describes one export.
type Request struct {
	ResumePath   string
	CachePath    string
	LoadoutsPath string
	Lang         string
	Profile      string
	// SaveLoadouts persists a profile that had to be created.
	SaveLoadouts bool
}

// Result is the exported document plus what happened on the way.
type Result struct {
	Doc        *resume.Doc
	Loadout    loadout.Loadout
	Translated int
	Missing    int
	Stats      i18n.Stats
}

// JSON renders the document as the bytes that get written or compiled.
func (r *Result) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(r.Doc.Root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode export: %w", err)
	}
	return append(b, '\n'), nil
}

// Run performs the export.
func Run(req Request) (*Result, error) {
	if req.Lang == "" {
		req.Lang = "en"
	}
	doc, err := resume.Load(req.ResumePath)
	if err != nil {
		return nil, err
	}
	cache, err := i18n.Load(req.CachePath)
	if err != nil {
		return nil, err
	}
	profiles, err := loadout.Load(req.LoadoutsPath)
	if err != nil {
		return nil, err
	}

	out := doc.Clone()
	res := &Result{Doc: out}

	if req.Profile != "" {
		l, ok := profiles.Get(req.Profile)
		if !ok {
			// A named profile that does not exist yet means "everything",
			// which keeps `--profile default` working on a fresh clone.
			l = loadout.Full(out, req.Profile)
			profiles.Put(l)
			if req.SaveLoadouts {
				if err := profiles.Save(req.LoadoutsPath); err != nil {
					return nil, err
				}
			}
		}
		res.Loadout = l
		loadout.Apply(out, l)
	}

	res.Translated, res.Missing = i18n.Apply(out, cache, req.Lang)
	res.Stats = cache.StatsFor(doc.Refs())
	return res, nil
}

// Write emits the export to OutPath (default stdout).
func (r *Result) Write(path string, stdout io.Writer) (string, error) {
	b, err := r.JSON()
	if err != nil {
		return "", err
	}
	if path == "" || path == "-" {
		if stdout == nil {
			stdout = os.Stdout
		}
		_, err := stdout.Write(b)
		return "-", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", fmt.Errorf("write export %s: %w", path, err)
	}
	return path, nil
}
