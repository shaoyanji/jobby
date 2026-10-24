package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shaoyanji/jobby/cmd/resumetui/views"
)

type buildDoneMsg struct {
	out string
	err error
}

type syncDoneMsg struct {
	n     int
	errs  []error
	cache Cache
}

type bulletItem struct {
	section  string
	group    string
	idx      int
	text     string
	selected bool
}

type model struct {
	cfg      Config
	resume   *Resume
	cache    Cache
	loadouts *LoadoutFile
	loadout  Loadout
	refs     []Ref
	items    []bulletItem
	cursors  [3]int
	tab      int
	editing  bool
	input    textinput.Model
	status   string
	pdfInfo  string
	width    int
}

func newModel(cfg Config) (*model, error) {
	r, err := LoadResume(cfg.Resume)
	if err != nil {
		return nil, err
	}
	c, err := LoadCache(cfg.Cache)
	if err != nil {
		return nil, err
	}
	lf, err := LoadLoadouts(cfg.Loadouts)
	if err != nil {
		return nil, err
	}
	l, ok := lf.Get(lf.Active)
	if !ok {
		l = Loadout{Name: "default", Language: "en"}
	}
	if l.Language == "" {
		l.Language = "en"
	}
	ti := textinput.New()
	ti.Placeholder = "German translation…"
	ti.CharLimit = 1024
	m := &model{
		cfg: cfg, resume: r, cache: c, loadouts: lf, loadout: l,
		input: ti, width: 100,
	}
	m.refs = r.Translatable()
	m.rebuildItems()
	return m, nil
}

func (m *model) rebuildItems() {
	m.items = nil
	selected := func(group string, idx int, hasKey bool, idxs []int) bool {
		if !hasKey {
			return true
		}
		for _, i := range idxs {
			if i == idx {
				return true
			}
		}
		return false
	}
	for i, w := range m.resume.Work {
		key := "work[" + itoa(i) + "]"
		idxs, has := m.loadout.Work[key]
		for j, h := range w.Highlights {
			m.items = append(m.items, bulletItem{
				section: w.Name + " — " + w.Position, group: key, idx: j, text: h,
				selected: selected(key, j, has, idxs),
			})
		}
	}
	for i, p := range m.resume.Projects {
		key := "projects[" + itoa(i) + "]"
		idxs, has := m.loadout.Projects[key]
		for j, h := range p.Highlights {
			m.items = append(m.items, bulletItem{
				section: "Projects — " + p.Name, group: key, idx: j, text: h,
				selected: selected(key, j, has, idxs),
			})
		}
	}
}

func (m *model) syncLoadout() {
	work := map[string][]int{}
	proj := map[string][]int{}
	for _, it := range m.items {
		if !it.selected {
			continue
		}
		if strings.HasPrefix(it.group, "work[") {
			work[it.group] = append(work[it.group], it.idx)
		} else {
			proj[it.group] = append(proj[it.group], it.idx)
		}
	}
	m.loadout.Work = work
	m.loadout.Projects = proj
	m.loadouts.Put(m.loadout)
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case buildDoneMsg:
		if msg.err != nil {
			m.status = views.Bad.Render("build failed: " + msg.err.Error())
		} else {
			m.pdfInfo = Info(m.pdfPath())
			m.status = views.Good.Render("build ok · " + m.pdfPath())
		}
		return m, nil
	case syncDoneMsg:
		if msg.cache != nil {
			m.cache = msg.cache
		}
		if err := m.cache.Save(m.cfg.Cache); err != nil {
			m.status = views.Bad.Render("cache save: " + err.Error())
		} else {
			m.status = views.Good.Render(fmt.Sprintf("translated %d strings", msg.n))
		}
		if len(msg.errs) > 0 {
			m.status += views.Warn.Render(fmt.Sprintf(" · %d errors", len(msg.errs)))
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	if m.editing {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if m.editing {
		switch k {
		case "esc":
			m.editing = false
			m.input.Blur()
			return m, nil
		case "enter":
			row := m.refs[m.clampCursor()]
			m.cache.SetHuman(row.EN, strings.TrimSpace(m.input.Value()))
			_ = m.cache.Save(m.cfg.Cache)
			m.editing = false
			m.input.Blur()
			m.status = views.Good.Render("saved translation")
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch k {
	case "q", "ctrl+c":
		m.syncLoadout()
		_ = m.loadouts.Save(m.cfg.Loadouts)
		return m, tea.Quit
	case "1", "2", "3":
		m.tab = int(k[0] - '1')
	case "tab":
		m.tab = (m.tab + 1) % 3
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "l":
		if m.loadout.Language == "de" {
			m.loadout.Language = "en"
		} else {
			m.loadout.Language = "de"
		}
		m.syncLoadout()
		m.status = views.Sel.Render("language " + strings.ToUpper(m.loadout.Language))
	case "s":
		m.syncLoadout()
		if err := m.loadouts.Save(m.cfg.Loadouts); err != nil {
			m.status = views.Bad.Render(err.Error())
		} else {
			m.status = views.Good.Render("loadout saved: " + m.loadout.Name)
		}
	case " ":
		if m.tab == 0 && len(m.items) > 0 {
			i := m.cursors[0]
			m.items[i].selected = !m.items[i].selected
			m.syncLoadout()
		}
	case "enter":
		if m.tab == 0 && len(m.items) > 0 {
			i := m.cursors[0]
			m.items[i].selected = !m.items[i].selected
			m.syncLoadout()
		}
		if m.tab == 1 && len(m.refs) > 0 {
			row := m.refs[m.clampCursor()]
			m.input.SetValue(m.cache[row.EN].DEHuman)
			m.input.Focus()
			m.editing = true
		}
	case "t":
		if m.tab == 1 && len(m.refs) > 0 {
			row := m.refs[m.clampCursor()]
			return m, m.syncCmd([]Ref{row})
		}
	case "T":
		if m.tab == 1 {
			return m, m.syncCmd(m.refs)
		}
	case "b":
		lang := m.loadout.Language
		m.status = views.Dim.Render("building…")
		return m, func() tea.Msg {
			out, err := Build(lang)
			return buildDoneMsg{out: out, err: err}
		}
	case "m":
		lang := m.loadout.Language
		m.status = views.Dim.Render("merging Bewerbungsmappe…")
		return m, func() tea.Msg {
			out, err := Mappe(lang)
			return buildDoneMsg{out: out, err: err}
		}
	case "p":
		if err := Preview(m.pdfPath()); err != nil {
			m.status = views.Bad.Render(err.Error())
		}
	}
	return m, nil
}

func (m *model) syncCmd(refs []Ref) tea.Cmd {
	c := Cache{}
	for k, v := range m.cache {
		c[k] = v
	}
	return func() tea.Msg {
		n, errs := c.Sync(refs, nil)
		return syncDoneMsg{n: n, errs: errs, cache: c}
	}
}

func (m *model) move(delta int) {
	limit := 0
	switch m.tab {
	case 0:
		limit = len(m.items)
	case 1:
		limit = len(m.refs)
	}
	if limit == 0 {
		return
	}
	i := m.cursors[m.tab] + delta
	if i < 0 {
		i = 0
	}
	if i >= limit {
		i = limit - 1
	}
	m.cursors[m.tab] = i
}

func (m *model) clampCursor() int {
	i := m.cursors[1]
	if i >= len(m.refs) {
		i = len(m.refs) - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

func (m *model) pdfPath() string {
	return filepath.Join(m.cfg.OutDir, "resume-"+m.loadout.Language+".pdf")
}

func (m *model) View() string {
	verified, auto, missing := m.cache.Coverage(m.refs)
	h := views.Header{
		Loadout: m.loadout.Name, Lang: m.loadout.Language,
		Verified: verified, Total: len(m.refs), PDF: m.pdfLine(),
	}
	var body string
	switch m.tab {
	case 0:
		items := make([]views.Item, 0, len(m.items))
		for _, it := range m.items {
			items = append(items, views.Item{Section: it.section, Text: it.text, Selected: it.selected})
		}
		body = views.Loadout(views.LoadoutView{Items: items, Cursor: m.cursors[0], Label: m.resume.Basics.Label})
	case 1:
		rows := make([]views.Row, 0, len(m.refs))
		for _, ref := range m.refs {
			e := m.cache[contentKey(ref.EN)]
			de := e.DEHuman
			if de == "" {
				de = e.DEAuto
			}
			rows = append(rows, views.Row{Path: ref.Path, EN: ref.EN, DE: de, Status: m.cache.Status(ref.EN)})
		}
		editor := ""
		if m.editing {
			editor = m.input.View()
		}
		body = views.I18n(views.I18nView{
			Rows: rows, Cursor: m.cursors[1], Editing: m.editing, Editor: editor,
			Coverage: fmt.Sprintf("%d verified · %d auto · %d missing", verified, auto, missing),
		})
	case 2:
		body = views.PDF(views.PDFView{
			Toggles: []views.Toggle{
				{Label: "Stamp signature.png on last page", On: true},
				{Label: "Embed active JSON as PDF attachment", On: true},
				{Label: "Inject SyncedTask + git revision into properties", On: true},
				{Label: "Merge Bewerbungsmappe (anschreiben + CV + zeugnisse)", On: false},
			},
			Info: m.pdfInfo, Status: m.status,
		})
	}
	status := m.status
	if status == "" {
		status = views.Dim.Render("ready")
	}
	footer := views.Dim.Render("1/2/3 tabs · j/k move · space toggle · t translate · T all · l language · b build · p preview · m mappe · s save · q quit")
	return lipgloss.JoinVertical(lipgloss.Left, h.Render(m.tab), body, "", status, footer)
}

func (m *model) pdfLine() string {
	if m.pdfInfo == "" {
		return "pdf: not built"
	}
	for _, line := range strings.Split(m.pdfInfo, "\n") {
		if strings.Contains(strings.ToLower(line), "page count") {
			return strings.TrimSpace(line)
		}
	}
	return "pdf: built"
}
