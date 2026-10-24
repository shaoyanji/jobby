// Package layout compiles a resume or cover letter into a pdfcpu `create`
// document. pdfcpu only draws at explicit coordinates, so this package owns the
// flow: it measures text, wraps it, advances a y cursor, breaks pages and hands
// pdfcpu a flat list of positioned text and image boxes.
package layout

import "strings"

// face is one of the Helvetica variants the compiler can measure. The widths in
// metrics_helvetica.go come from the same Core14 AFMs pdfcpu renders with, so a
// line measured to fit does fit.
type face int

const (
	helvetica face = iota
	helveticaBold
	helveticaOblique
)

func (f face) Name() string {
	switch f {
	case helveticaBold:
		return "Helvetica-Bold"
	case helveticaOblique:
		return "Helvetica-Oblique"
	default:
		return "Helvetica"
	}
}

// Measure returns the advance width of s in points.
func Measure(s string, size float64, f face) float64 {
	table := winAnsiWidths[f]
	total := 0.0
	for _, r := range s {
		c := winAnsiCode(r)
		w := table[c]
		if w == 0 {
			w = table[int('?')]
		}
		total += float64(w)
	}
	return total / 1000 * size
}

// Wrap breaks s into lines no wider than width, honouring explicit newlines.
// Words longer than the line are split by character so a stray URL can never
// overflow the margin.
func Wrap(s string, width, size float64, f face) []string {
	var out []string
	for _, paragraph := range strings.Split(Sanitize(s), "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if Measure(try, size, f) <= width || line == "" {
				line = try
				for Measure(line, size, f) > width {
					// A single word wider than the column: cut it.
					cut := len(line)
					for cut > 1 && Measure(line[:cut], size, f) > width {
						cut--
					}
					out = append(out, line[:cut])
					line = line[cut:]
				}
				continue
			}
			out = append(out, line)
			line = word
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// Sanitize maps typographic characters the Core14 WinAnsi fonts cannot encode
// onto their plain equivalents instead of letting them render as a blank.
func Sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\u00a0', '\u2007', '\u202f':
			b.WriteByte(' ')
		case '\u2192':
			b.WriteString("->")
		case '\u2190':
			b.WriteString("<-")
		case '\u2026':
			b.WriteString("...")
		case '\u2011':
			b.WriteByte('-')
		default:
			if r < 256 {
				b.WriteRune(r)
				continue
			}
			if _, ok := cp1252[r]; ok {
				b.WriteRune(r)
				continue
			}
			b.WriteByte('?')
		}
	}
	return b.String()
}

// winAnsiCode maps a rune to its CP1252 code. Runes outside CP1252 fall back to
// '?' so measurement and rendering agree.
func winAnsiCode(r rune) int {
	if r >= 32 && r < 127 {
		return int(r)
	}
	if c, ok := cp1252[r]; ok {
		return c
	}
	return int('?')
}

var cp1252 = map[rune]int{
	'\u20ac': 128, '\u201a': 130, '\u0192': 131, '\u201e': 132,
	'\u2026': 133, '\u2020': 134, '\u2021': 135, '\u02c6': 136,
	'\u2030': 137, '\u0160': 138, '\u2039': 139, '\u0152': 140,
	'\u017d': 142, '\u2018': 145, '\u2019': 146, '\u201c': 147,
	'\u201d': 148, '\u2022': 149, '\u2013': 150, '\u2014': 151,
	'\u02dc': 152, '\u2122': 153, '\u0161': 154, '\u203a': 155,
	'\u0153': 156, '\u017e': 158, '\u0178': 159,
}
