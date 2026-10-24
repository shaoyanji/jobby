package views

import (
	"fmt"
	"strings"
)

type Item struct {
	Section  string
	Text     string
	Selected bool
}

type LoadoutView struct {
	Items  []Item
	Cursor int
	Label  string
}

// Loadout renders the checklist plus a rough 2-page density gauge (A4 fits
// roughly 55 CV bullets; the bar turns red past 90%).
func Loadout(v LoadoutView) string {
	var b strings.Builder
	b.WriteString(Title.Render("Deckbuilder") + Dim.Render("  space toggle · j/k move · enter toggle · s save") + "\n\n")
	if v.Label != "" {
		b.WriteString(Dim.Render("persona  ") + Sel.Render(v.Label) + "\n\n")
	}
	section := ""
	selected := 0
	for i, it := range v.Items {
		if it.Section != section {
			section = it.Section
			b.WriteString(Title.Render(section) + "\n")
		}
		check := "[ ]"
		if it.Selected {
			check = "[x]"
			selected++
		}
		line := fmt.Sprintf("  %s %s", check, it.Text)
		if i == v.Cursor {
			b.WriteString(Sel.Render("› "+line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	if len(v.Items) == 0 {
		b.WriteString(Dim.Render("no highlights found in resume.json") + "\n")
	}
	b.WriteString("\n" + gauge(selected, 55))
	return b.String()
}

func gauge(selected, budget int) string {
	if budget <= 0 {
		budget = 55
	}
	ratio := float64(selected) / float64(budget)
	filled := int(ratio * 30)
	if filled > 30 {
		filled = 30
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 30-filled)
	style := Good
	if ratio >= 1 {
		style = Bad
	} else if ratio >= 0.9 {
		style = Warn
	}
	return Dim.Render("density  ") + style.Render(bar) +
		Dim.Render(fmt.Sprintf("  %d/%d bullets", selected, budget))
}
