package main

import (
	"path/filepath"
	"testing"
)

func TestApplyLoadoutKeepsChosenOrder(t *testing.T) {
	r := &Resume{
		Basics: Basics{Label: "original", AltLabels: []string{"a"}},
		Work: []Work{{
			Name:       "ACME",
			Highlights: []string{"zero", "one", "two"},
		}},
		Projects: []Project{{Name: "P", Highlights: []string{"p0", "p1"}}},
		Meta:     map[string]any{},
	}
	l := Loadout{
		Name:  "energy-de",
		Label: "Energieingenieur",
		Work:  map[string][]int{"work[0]": {2, 0}},
	}
	ApplyLoadout(r, l)

	if got := r.Work[0].Highlights; len(got) != 2 || got[0] != "two" || got[1] != "zero" {
		t.Errorf("highlights = %v, want [two zero]", got)
	}
	if r.Basics.Label != "Energieingenieur" {
		t.Errorf("label = %q", r.Basics.Label)
	}
	if len(r.Projects[0].Highlights) != 2 {
		t.Errorf("projects without a loadout entry must be untouched, got %v", r.Projects[0].Highlights)
	}
}

func TestLoadoutsRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "loadouts.json")
	lf := &LoadoutFile{Version: 1, Loadouts: map[string]Loadout{}}
	lf.Put(Loadout{Name: "ai-en", Language: "en", Work: map[string][]int{"work[0]": {1}}})
	if err := lf.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLoadouts(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active != "ai-en" {
		t.Errorf("active = %q, want ai-en", got.Active)
	}
	l, ok := got.Get("ai-en")
	if !ok || l.Work["work[0]"][0] != 1 {
		t.Errorf("loadout did not round-trip: %+v", l)
	}
}
