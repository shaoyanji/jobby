package views

import (
	"strings"
)

type Row struct {
	Path   string
	EN     string
	DE     string
	Status string
}

type I18nView struct {
	Rows     []Row
	Cursor   int
	Editing  bool
	Editor   string
	Coverage string
}

func I18n(v I18nView) string {
	var b strings.Builder
	b.WriteString(Title.Render("Bilingual review") + Dim.Render("  j/k move · t translate · T translate all · enter edit/save · esc cancel") + "\n")
	if v.Coverage != "" {
		b.WriteString(Dim.Render(v.Coverage) + "\n")
	}
	b.WriteString("\n")
	start := 0
	if v.Cursor > 8 {
		start = v.Cursor - 8
	}
	for i := start; i < len(v.Rows) && i < start+16; i++ {
		r := v.Rows[i]
		prefix := "  "
		if i == v.Cursor {
			prefix = "› "
		}
		de := r.DE
		if de == "" {
			de = Dim.Render("—")
		}
		line := prefix + Badge(r.Status) + " " + Dim.Render(r.Path)
		if i == v.Cursor {
			b.WriteString(Sel.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
		b.WriteString("    " + Dim.Render("EN ") + truncate(r.EN, 72) + "\n")
		b.WriteString("    " + Dim.Render("DE ") + truncate(de, 72) + "\n")
	}
	if len(v.Rows) == 0 {
		b.WriteString(Dim.Render("no translatable strings") + "\n")
	}
	if v.Editing {
		b.WriteString("\n" + Title.Render("German translation") + "\n")
		b.WriteString(v.Editor + "\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
