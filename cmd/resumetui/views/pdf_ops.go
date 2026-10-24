package views

import "strings"

type Toggle struct {
	Label string
	On    bool
}

type PDFView struct {
	Toggles []Toggle
	Info    string
	Status  string
}

func PDF(v PDFView) string {
	var b strings.Builder
	b.WriteString(Title.Render("PDF cockpit") + Dim.Render("  b build · p preview · l switch language · m mappe") + "\n\n")
	for _, t := range v.Toggles {
		box := "[ ]"
		if t.On {
			box = "[x]"
		}
		b.WriteString("  " + box + " " + t.Label + "\n")
	}
	b.WriteString("\n" + Title.Render("pdfcpu info") + "\n")
	if v.Info == "" {
		b.WriteString(Dim.Render("  (no build yet)") + "\n")
	} else {
		for _, line := range strings.Split(v.Info, "\n") {
			b.WriteString("  " + line + "\n")
		}
	}
	if v.Status != "" {
		b.WriteString("\n" + v.Status + "\n")
	}
	return b.String()
}
