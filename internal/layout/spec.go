package layout

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Spec is a pdfcpu `create` document: paper, origin, optional layout guides and
// one flat content list per page. Every element carries its own position, which
// is what makes the output deterministic.
type Spec struct {
	Paper      string          `json:"paper"`
	Origin     string          `json:"origin"`
	Guides     bool            `json:"guides,omitempty"`
	ContentBox bool            `json:"contentBox,omitempty"`
	Fonts      map[string]Font `json:"fonts"`
	Pages      map[string]Page `json:"pages"`
}

// Font registers a global font so text boxes can refer to it by name.
type Font struct {
	Name  string  `json:"name"`
	Size  float64 `json:"size"`
	Color string  `json:"col,omitempty"`
}

// Page is one page's content.
type Page struct {
	Content Content `json:"content"`
}

// Content holds positioned elements. The JSON keys are pdfcpu's: note that image
// boxes live under "image" and plain rectangles under "box".
type Content struct {
	Text   []TextBox   `json:"text,omitempty"`
	Images []ImageBox  `json:"image,omitempty"`
	Boxes  []SimpleBox `json:"box,omitempty"`
}

// TextBox is one positioned string. pdfcpu wraps at width and aligns within it,
// but the compiler wraps first so the y grid stays exact; align is only used for
// the right-hand date column, where pos.x is the right edge.
type TextBox struct {
	Value string     `json:"value"`
	Pos   [2]float64 `json:"pos"`
	// Width is only set for right-aligned meta text, where pdfcpu needs a box.
	Width float64 `json:"width,omitempty"`
	Align string  `json:"align,omitempty"`
	Font  Font    `json:"font"`
}

// ImageBox draws an image. pdfcpu resolves a relative src against its working
// directory, so the compiler always emits a root-relative path.
type ImageBox struct {
	Src   string     `json:"src"`
	Pos   [2]float64 `json:"pos"`
	Width float64    `json:"width,omitempty"`
}

// SimpleBox is a filled rectangle: rules under headings and the header band.
type SimpleBox struct {
	Pos     [2]float64 `json:"pos"`
	Width   float64    `json:"width"`
	Height  float64    `json:"height"`
	FillCol string     `json:"fillCol,omitempty"`
}

// WriteJSON writes the spec atomically: a half-written layout file would make
// the next `pdfcpu create` fail with a confusing parse error.
func (s *Spec) WriteJSON(path string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ContentWidth is the usable width between the margins.
const (
	PageWidth  = 595.0
	PageHeight = 842.0

	MarginLeft   = 52.0
	MarginRight  = 52.0
	MarginTop    = 46.0
	MarginBottom = 52.0

	// MetaWidth is the date/location column; it is right-aligned to the page's
	// right margin so a long German job title can never push a date inward.
	MetaWidth = 125.0
	MetaGap   = 10.0
)

// LeftWidth is the width available to the left column of an entry.
const LeftWidth = PageWidth - MarginLeft - MarginRight - MetaWidth - MetaGap

// rightEdge is where right-aligned meta text ends.
const rightEdge = PageWidth - MarginRight

// Density is one typography preset. The solver walks the list until the page
// count fits the budget, so the CV is never tighter than it has to be.
type Density struct {
	ID         string
	Body       float64
	Small      float64
	Heading    float64
	Name       float64
	LineHeight float64
	SectionGap float64
	EntryGap   float64
	BulletGap  float64
}

// Densities run from comfortable to compact.
var Densities = []Density{
	{
		ID: "10.0pt", Body: 10.0, Small: 8.8, Heading: 9.6, Name: 18.0,
		LineHeight: 12.4, SectionGap: 13.0, EntryGap: 7.0, BulletGap: 3.4,
	},
	{
		ID: "9.5pt", Body: 9.5, Small: 8.4, Heading: 9.2, Name: 17.5,
		LineHeight: 11.7, SectionGap: 10.0, EntryGap: 5.5, BulletGap: 2.6,
	},
	{
		ID: "9.0pt", Body: 9.0, Small: 8.0, Heading: 8.8, Name: 17.0,
		LineHeight: 11.0, SectionGap: 8.0, EntryGap: 4.0, BulletGap: 1.8,
	},
}

// Default colors, muted enough to read as one stationery set.
const (
	Ink     = "#1a1a1a"
	Muted   = "#5f6368"
	Rule    = "#c9ccd1"
	Heading = "#111111"
)
