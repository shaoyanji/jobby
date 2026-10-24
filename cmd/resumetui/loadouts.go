package main

import (
	"encoding/json"
	"errors"
	"os"
)

// Loadout is a saved deck: one persona label plus the ordered highlight indices
// that survive the export. Nothing here deletes content from resume.json.
type Loadout struct {
	Name     string           `json:"name"`
	Language string           `json:"language"`
	Label    string           `json:"label,omitempty"`
	Work     map[string][]int `json:"work,omitempty"`
	Projects map[string][]int `json:"projects,omitempty"`
}

type LoadoutFile struct {
	Version  int                `json:"version"`
	Active   string             `json:"active"`
	Loadouts map[string]Loadout `json:"loadouts"`
}

func LoadLoadouts(path string) (*LoadoutFile, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &LoadoutFile{Version: 1, Loadouts: map[string]Loadout{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var lf LoadoutFile
	if err := json.Unmarshal(b, &lf); err != nil {
		return nil, err
	}
	if lf.Loadouts == nil {
		lf.Loadouts = map[string]Loadout{}
	}
	return &lf, nil
}

func (lf *LoadoutFile) Save(path string) error {
	b, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func (lf *LoadoutFile) Get(name string) (Loadout, bool) {
	l, ok := lf.Loadouts[name]
	return l, ok
}

func (lf *LoadoutFile) Put(l Loadout) {
	if lf.Loadouts == nil {
		lf.Loadouts = map[string]Loadout{}
	}
	lf.Loadouts[l.Name] = l
	lf.Active = l.Name
}

// ApplyLoadout rewrites the exported view: the persona label, the active
// language, and only the selected bullet indices in their chosen order.
func ApplyLoadout(r *Resume, l Loadout) {
	if l.Label != "" {
		r.Basics.Label = l.Label
	}
	for i := range r.Work {
		key := "work[" + itoa(i) + "]"
		if idx, ok := l.Work[key]; ok {
			r.Work[i].Highlights = pick(r.Work[i].Highlights, idx)
		}
	}
	for i := range r.Projects {
		key := "projects[" + itoa(i) + "]"
		if idx, ok := l.Projects[key]; ok {
			r.Projects[i].Highlights = pick(r.Projects[i].Highlights, idx)
		}
	}
}

func pick(all []string, idx []int) []string {
	out := make([]string, 0, len(idx))
	for _, i := range idx {
		if i >= 0 && i < len(all) {
			out = append(out, all[i])
		}
	}
	return out
}
