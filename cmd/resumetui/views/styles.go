package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	Accent  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	Dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	Good    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	Warn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	Bad     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	Sel     = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	Title   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	Box     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	TabOn   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("81")).Padding(0, 1)
	TabOff  = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 1)
	BadgeOK = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	BadgeAU = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	BadgeMS = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type Header struct {
	Loadout  string
	Lang     string
	Verified int
	Total    int
	PDF      string
}

func (h Header) Render(tab int) string {
	name := h.Loadout
	if name == "" {
		name = "unsaved"
	}
	lang := strings.ToUpper(h.Lang)
	if lang == "" {
		lang = "EN"
	}
	left := Accent.Render("resumetui") + Dim.Render("  ·  ") + name +
		Dim.Render("  ·  ") + Sel.Render("["+lang+"]") +
		Dim.Render(fmt.Sprintf("  ·  %d/%d verified", h.Verified, h.Total))
	right := Dim.Render(h.PDF)
	bar := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)

	tabs := []string{"1 Loadout", "2 Bilingual", "3 PDF"}
	rendered := make([]string, len(tabs))
	for i, t := range tabs {
		if i == tab {
			rendered[i] = TabOn.Render(t)
		} else {
			rendered[i] = TabOff.Render(t)
		}
	}
	return bar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, rendered...) + "\n"
}

func Badge(status string) string {
	switch status {
	case "verified":
		return BadgeOK.Render("[VERIFIED]")
	case "auto":
		return BadgeAU.Render("[AUTO-DE]")
	default:
		return BadgeMS.Render("[MISSING]")
	}
}
