package pdf

import "fmt"

// Preset is one typography density the auto-fit loop can try. The values are
// read by templates/resume.typ, so a preset is a layout decision, not a magic
// string in Go.
type Preset struct {
	FontSize float64 `json:"font_size"`
	VSpacing float64 `json:"v_spacing"`
	Label    string  `json:"label"`
}

// presets run from comfortable to dense. The list is ordered: the solver stops
// at the first one that fits the page budget, so the CV is never tighter than
// it has to be.
var presets = []Preset{
	{FontSize: 10.0, VSpacing: 0.65, Label: "10pt / normal"},
	{FontSize: 9.6, VSpacing: 0.55, Label: "9.6pt / compact"},
	{FontSize: 9.2, VSpacing: 0.45, Label: "9.2pt / tight"},
	{FontSize: 8.9, VSpacing: 0.38, Label: "8.9pt / dense"},
}

// Presets returns a copy of the density ladder.
func Presets() []Preset {
	out := make([]Preset, len(presets))
	copy(out, presets)
	return out
}

// Attempt records one compile in the fit loop, so a UI can explain what
// happened ("tried 10pt: 3 pages; 9.6pt: 2 pages").
type Attempt struct {
	Preset Preset
	Pages  int
	Err    string
}

// FitResult is the outcome of the loop.
type FitResult struct {
	Preset   Preset
	Pages    int
	Budget   int
	Fitted   bool
	Attempts []Attempt
}

// Fit compiles with each preset until the page count fits the budget.
//
// A compile error is fatal and returned: a broken template will not fix itself
// by shrinking the font. Running out of presets is not fatal; the result is the
// densest layout with Fitted=false so the caller can warn and still ship.
func Fit(
	budget int,
	compile func(Preset) (string, error),
	pagesOf func(string) (int, error),
) (FitResult, error) {
	if budget <= 0 {
		budget = 2
	}
	res := FitResult{Budget: budget}
	for _, preset := range presets {
		pdf, err := compile(preset)
		if err != nil {
			res.Attempts = append(res.Attempts, Attempt{Preset: preset, Err: err.Error()})
			return res, err
		}
		pages, err := pagesOf(pdf)
		if err != nil {
			res.Attempts = append(res.Attempts, Attempt{Preset: preset, Err: err.Error()})
			return res, fmt.Errorf("read page count for %s: %w", preset.Label, err)
		}
		res.Attempts = append(res.Attempts, Attempt{Preset: preset, Pages: pages})
		res.Preset = preset
		res.Pages = pages
		if pages <= budget {
			res.Fitted = true
			return res, nil
		}
	}
	return res, nil
}
