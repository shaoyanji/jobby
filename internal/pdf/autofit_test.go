package pdf

import (
	"errors"
	"testing"
)

// fakeCompiler returns canned page counts in order.
func fakeCompiler(pages []int) (func(Preset) (string, error), func(string) (int, error)) {
	i := 0
	compile := func(p Preset) (string, error) {
		name := p.Label
		if i >= len(pages) {
			return name, nil
		}
		i++
		return name, nil
	}
	read := func(string) (int, error) {
		n := pages[0]
		if len(pages) > 1 {
			pages = pages[1:]
		}
		return n, nil
	}
	return compile, read
}

func TestFitStopsAtTheFirstPresetThatFits(t *testing.T) {
	compile, read := fakeCompiler([]int{2})
	res, err := Fit(2, compile, read)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Fitted || res.Pages != 2 {
		t.Fatalf("res = %+v", res)
	}
	if len(res.Attempts) != 1 {
		t.Errorf("attempts = %d, want 1 (comfortable preset already fit)", len(res.Attempts))
	}
	if res.Preset.Label != "10pt / normal" {
		t.Errorf("preset = %q", res.Preset.Label)
	}
}

func TestFitTightensOneStep(t *testing.T) {
	compile, read := fakeCompiler([]int{3, 2})
	res, err := Fit(2, compile, read)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Fitted || res.Preset.Label != "9.6pt / compact" {
		t.Fatalf("res = %+v", res)
	}
	if len(res.Attempts) != 2 {
		t.Errorf("attempts = %+v", res.Attempts)
	}
}

func TestFitExhaustsTheLadderWithoutFailing(t *testing.T) {
	compile, read := fakeCompiler([]int{5, 4, 3, 3})
	res, err := Fit(1, compile, read)
	if err != nil {
		t.Fatalf("running out of presets must not be fatal: %v", err)
	}
	if res.Fitted {
		t.Error("reported fitted at 3 pages against a 1-page budget")
	}
	if res.Preset.Label != "8.9pt / dense" {
		t.Errorf("preset = %q, want the densest", res.Preset.Label)
	}
	if len(res.Attempts) != 4 {
		t.Errorf("attempts = %d", len(res.Attempts))
	}
}

func TestFitTreatsCompileErrorsAsFatal(t *testing.T) {
	boom := errors.New("typst: unknown field")
	compile := func(Preset) (string, error) { return "", boom }
	_, err := Fit(2, compile, func(string) (int, error) { return 1, nil })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestFitNeedsAPositiveBudget(t *testing.T) {
	compile, read := fakeCompiler([]int{2})
	res, err := Fit(0, compile, read)
	if err != nil {
		t.Fatal(err)
	}
	if res.Budget != 2 {
		t.Errorf("budget = %d, want the default 2", res.Budget)
	}
}

func TestPresetsAreOrderedComfortableToDense(t *testing.T) {
	p := Presets()
	if len(p) != 4 {
		t.Fatalf("presets = %d", len(p))
	}
	for i := 1; i < len(p); i++ {
		if p[i].FontSize >= p[i-1].FontSize {
			t.Errorf("preset %d is not denser than %d", i, i-1)
		}
	}
	if p[1].Label != "9.6pt / compact" {
		t.Errorf("label = %q", p[1].Label)
	}
}
