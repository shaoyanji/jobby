package layout

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"time"

	"github.com/shaoyanji/jobby/internal/coverletter"
	"github.com/shaoyanji/jobby/internal/resume"
)

// Options configures one compile.
type Options struct {
	Lang string
	// Guides turns on pdfcpu's gridlines and the content box, and highlights
	// every element, so alignment can be inspected in the viewer.
	Guides     bool
	ContentBox bool
	// Budget is the page target for the solver. Zero means 2; a forced Density
	// skips the solver.
	Budget  int
	Density *Density
	// SigSrc is the signature image as pdfcpu should resolve it (relative to
	// the working directory). SigFile is the same file on disk, used to read
	// its aspect ratio.
	SigSrc  string
	SigFile string
	SigW    float64
}

// Attempt records one density the solver tried.
type Attempt struct {
	Density string
	Pages   int
}

// Result is a compiled document after the page-budget solver ran.
type Result struct {
	Spec     *Spec
	Pages    int
	Density  Density
	Budget   int
	Fits     bool
	Attempts []Attempt
}

// labels are the section titles, so the compiled German CV is not an English
// document with translated bullets.
type labels struct {
	Profile, Experience, Projects, Skills, Education, Languages, Page string
}

func labelsFor(lang string) labels {
	if strings.HasPrefix(strings.ToLower(lang), "de") {
		return labels{
			Profile:    "Profil",
			Experience: "Berufserfahrung",
			Projects:   "Projekte",
			Skills:     "Kenntnisse",
			Education:  "Ausbildung",
			Languages:  "Sprachen",
			Page:       "Seite %d von %d",
		}
	}
	return labels{
		Profile:    "Profile",
		Experience: "Experience",
		Projects:   "Projects",
		Skills:     "Skills",
		Education:  "Education",
		Languages:  "Languages",
		Page:       "page %d of %d",
	}
}

// Compile lays out the CV and solves for the page budget in memory. Nothing is
// written: the caller gets the spec and can either dump it or hand it straight
// to pdfcpu.
func Compile(doc *resume.Doc, o Options) Result {
	if o.Budget <= 0 {
		o.Budget = 2
	}
	if o.Density != nil {
		spec, pages := compileCV(doc, o, *o.Density)
		return Result{
			Spec: spec, Pages: pages, Density: *o.Density,
			Fits:     pages <= o.Budget,
			Attempts: []Attempt{{Density: o.Density.ID, Pages: pages}},
		}
	}
	res := Result{Budget: o.Budget}
	for _, d := range Densities {
		spec, pages := compileCV(doc, o, d)
		res.Attempts = append(res.Attempts, Attempt{Density: d.ID, Pages: pages})
		res.Spec, res.Pages, res.Density = spec, pages, d
		if pages <= o.Budget {
			res.Fits = true
			return res
		}
	}
	return res
}

// CompileLetter lays the cover letter out on the same stationery. A letter is
// one page by design, so the solver only tightens when it has to.
func CompileLetter(doc *resume.Doc, l coverletter.Letter, o Options) Result {
	if o.Budget <= 0 {
		o.Budget = 1
	}
	if o.Density != nil {
		spec, pages := compileLetter(doc, l, o, *o.Density)
		return Result{
			Spec: spec, Pages: pages, Density: *o.Density,
			Fits:     pages <= o.Budget,
			Attempts: []Attempt{{Density: o.Density.ID, Pages: pages}},
		}
	}
	res := Result{Budget: o.Budget}
	for _, d := range Densities {
		spec, pages := compileLetter(doc, l, o, d)
		res.Attempts = append(res.Attempts, Attempt{Density: d.ID, Pages: pages})
		res.Spec, res.Pages, res.Density = spec, pages, d
		if pages <= o.Budget {
			res.Fits = true
			return res
		}
	}
	return res
}

func compileCV(doc *resume.Doc, o Options, d Density) (*Spec, int) {
	p := newPainter(d, o)
	p.header(doc)
	p.section(labelsFor(o.Lang).Profile, docString(doc, "basics.summary"))

	if items := doc.Objects("work"); len(items) > 0 {
		p.sectionHead(labelsFor(o.Lang).Experience)
		for _, item := range items {
			p.entry(entryData{
				head:     stringField(item, "name"),
				detail:   stringField(item, "position"),
				dates:    dateRange(stringField(item, "startDate"), stringField(item, "endDate"), p.o.Lang),
				location: stringField(item, "location"),
				summary:  stringField(item, "summary"),
				bullets:  stringSlice(item, "highlights"),
			})
		}
	}
	if items := doc.Objects("projects"); len(items) > 0 {
		p.sectionHead(labelsFor(o.Lang).Projects)
		for _, item := range items {
			p.projectEntry(item)
		}
	}
	if rows := skillRows(doc); len(rows) > 0 {
		p.sectionHead(labelsFor(o.Lang).Skills)
		for _, row := range rows {
			p.inline(row.left, row.detail)
		}
		p.y += d.EntryGap
	}
	if items := doc.Objects("education"); len(items) > 0 {
		p.sectionHead(labelsFor(o.Lang).Education)
		for _, item := range items {
			// studyType already folds the area in; joining again printed it twice.
			p.twoColumn(stringField(item, "institution"), studyType(item),
				dateRange(stringField(item, "startDate"), stringField(item, "endDate"), p.o.Lang))
		}
	}
	if rows := languageRows(doc); len(rows) > 0 {
		p.sectionHead(labelsFor(o.Lang).Languages)
		for _, row := range rows {
			p.inline(row.left, row.detail)
		}
	}
	p.finish(doc, labelsFor(o.Lang))
	return p.spec, p.page
}

func compileLetter(doc *resume.Doc, l coverletter.Letter, o Options, d Density) (*Spec, int) {
	p := newPainter(d, o)
	p.header(doc)
	p.y += d.EntryGap

	// Company and date on one grid row; the role sits under the company in the
	// same muted colour the CV uses for secondary text.
	p.ensure(d.LineHeight * 2)
	p.meta(rightEdge, p.y, dateLabel(doc, o.Lang))
	p.text(MarginLeft, p.y, l.Company, helveticaBold, d.Body, Ink)
	p.y += d.LineHeight
	if role := strings.TrimSpace(l.Role); role != "" {
		for _, line := range Wrap(role, LeftWidth, d.Small, helvetica) {
			p.text(MarginLeft, p.y, line, helvetica, d.Small, Muted)
			p.y += d.LineHeight * 0.95
		}
	}
	p.y += d.EntryGap

	if s := strings.TrimSpace(l.Salutation); s != "" {
		p.block(s, helvetica, d.Body, Ink)
		p.y += d.LineHeight * 0.55
	}
	if s := strings.TrimSpace(l.Opening); s != "" {
		p.block(s, helvetica, d.Body, Ink)
		p.y += d.LineHeight * 0.55
	}
	for _, para := range l.Body {
		if strings.TrimSpace(para) == "" {
			continue
		}
		p.block(para, helvetica, d.Body, Ink)
		p.y += d.LineHeight * 0.55
	}
	if s := strings.TrimSpace(l.Closing); s != "" {
		p.block(s, helvetica, d.Body, Ink)
		p.y += d.EntryGap
	}
	p.finish(doc, labelsFor(o.Lang))
	return p.spec, p.page
}

// painter mutates one page at a time. y is the baseline of the next line, in
// points from the top edge (the spec's UpperLeft origin).
type painter struct {
	spec *Spec
	d    Density
	o    Options
	page int
	y    float64
	sigH float64
}

func newPainter(d Density, o Options) *painter {
	spec := &Spec{
		Paper:  "A4",
		Origin: "UpperLeft",
		Guides: o.Guides,
		// Guides are useless without the content box they measure against.
		ContentBox: o.ContentBox || o.Guides,
		Fonts: map[string]Font{
			"Helvetica":         {Name: "Helvetica", Size: d.Body},
			"Helvetica-Bold":    {Name: "Helvetica-Bold", Size: d.Body},
			"Helvetica-Oblique": {Name: "Helvetica-Oblique", Size: d.Body},
		},
		Pages: map[string]Page{},
	}
	p := &painter{spec: spec, d: d, o: o, page: 1, y: MarginTop + d.Name*0.95}
	// The signature sits above the bottom margin on the last page; reserving its
	// height on every page keeps text from ever colliding with the mark.
	if o.SigFile != "" {
		w := o.SigW
		if w == 0 {
			w = 92
		}
		if img, err := os.Open(o.SigFile); err == nil {
			cfg, _, derr := image.DecodeConfig(img)
			img.Close()
			if derr == nil && cfg.Width > 0 {
				p.sigH = w * float64(cfg.Height) / float64(cfg.Width)
			}
		}
	}
	return p
}

// bottom is the lowest y content may reach on any page.
func (p *painter) bottom() float64 {
	return PageHeight - MarginBottom - p.sigH - 4
}

// add appends an element to the current page. The map holds values, so the page
// has to be written back after every append.
func (p *painter) add(fn func(*Content)) {
	key := fmt.Sprint(p.page)
	pg := p.spec.Pages[key]
	fn(&pg.Content)
	p.spec.Pages[key] = pg
}

func (p *painter) newPage() {
	p.page++
	p.y = MarginTop + p.d.Name*0.95
}

// ensure breaks the page when height would not fit above the bottom margin.
func (p *painter) ensure(height float64) {
	if p.y+height > p.bottom() {
		p.newPage()
	}
}

func (p *painter) text(x, y float64, s string, f face, size float64, color string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	p.add(func(c *Content) {
		c.Text = append(c.Text, TextBox{
			Value: Sanitize(s),
			Pos:   [2]float64{x, y},
			Font:  Font{Name: f.Name(), Size: size, Color: color},
		})
	})
}

// meta writes right-aligned text whose right edge lands on the right margin.
func (p *painter) meta(x, y float64, s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	p.add(func(c *Content) {
		c.Text = append(c.Text, TextBox{
			Value: Sanitize(s),
			Pos:   [2]float64{x, y},
			Width: MetaWidth,
			Align: "right",
			Font:  Font{Name: helvetica.Name(), Size: p.d.Small, Color: Muted},
		})
	})
}

func (p *painter) rule(y, width float64) {
	p.add(func(c *Content) {
		c.Boxes = append(c.Boxes, SimpleBox{
			Pos:     [2]float64{MarginLeft, y},
			Width:   width,
			Height:  0.6,
			FillCol: Rule,
		})
	})
}

// drawLines writes wrapped lines, breaking pages line by line so a single long
// bullet can never run off the bottom edge.
func (p *painter) drawLines(x, width float64, s string, f face, size float64, color string) {
	lh := p.d.LineHeight * size / p.d.Body
	for _, line := range Wrap(s, width, size, f) {
		if p.y+lh > p.bottom() {
			p.newPage()
		}
		p.text(x, p.y, line, f, size, color)
		p.y += lh
	}
}

// block draws a paragraph at the left margin.
func (p *painter) block(s string, f face, size float64, color string) {
	p.drawLines(MarginLeft, PageWidth-MarginLeft-MarginRight, s, f, size, color)
}

// header draws the shared stationery: name, persona, alternative labels and the
// contact line, closed by a rule.
func (p *painter) header(doc *resume.Doc) {
	d := p.d
	name := doc.Name()
	p.text(MarginLeft, p.y, name, helveticaBold, d.Name, Ink)
	p.y += d.LineHeight * 1.25

	if label := docString(doc, "basics.label"); label != "" {
		lh := d.LineHeight * 0.95
		for _, line := range Wrap(label, PageWidth-MarginLeft-MarginRight, d.Body+0.5, helvetica) {
			p.text(MarginLeft, p.y, line, helvetica, d.Body+0.5, Heading)
			p.y += lh
		}
		p.y += 1.5
	}
	if alts := doc.Strings("basics.altLabels"); len(alts) > 0 {
		lh := d.LineHeight * 0.85
		for _, line := range Wrap(strings.Join(alts, " · "), PageWidth-MarginLeft-MarginRight, d.Small, helvetica) {
			p.text(MarginLeft, p.y, line, helvetica, d.Small, Muted)
			p.y += lh
		}
		p.y += 1.5
	}
	var contact []string
	for _, key := range []string{"basics.email", "basics.url", "basics.phone"} {
		if v := docString(doc, key); v != "" {
			contact = append(contact, v)
		}
	}
	if loc := docString(doc, "basics.location.city"); loc != "" {
		if cc := docString(doc, "basics.location.countryCode"); cc != "" {
			loc += " · " + cc
		}
		contact = append(contact, loc)
	}
	if len(contact) > 0 {
		for _, line := range Wrap(strings.Join(contact, " · "), PageWidth-MarginLeft-MarginRight, d.Small, helvetica) {
			p.text(MarginLeft, p.y, line, helvetica, d.Small, Muted)
			p.y += d.LineHeight * 0.85
		}
	}
	p.y += 4
	p.rule(p.y, PageWidth-MarginLeft-MarginRight)
	p.y += d.SectionGap
}

func (p *painter) sectionHead(title string) {
	if strings.TrimSpace(title) == "" {
		return
	}
	p.ensure(p.d.LineHeight*2 + p.d.SectionGap)
	p.text(MarginLeft, p.y, strings.ToUpper(title), helveticaBold, p.d.Heading, Heading)
	p.y += p.d.LineHeight * 0.7
	p.rule(p.y, PageWidth-MarginLeft-MarginRight)
	p.y += p.d.SectionGap * 0.75
}

// section is a heading plus one paragraph, skipped entirely when the paragraph
// is empty (a CV without a summary should not leave a floating title).
func (p *painter) section(title, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	p.sectionHead(title)
	p.block(body, helvetica, p.d.Body, Ink)
	p.y += p.d.SectionGap * 0.6
}

// entry draws one work or project item: a locked two-column head, then its
// summary and bullets.
// entryData is one CV entry, already reduced to the strings the grid draws.
// Work and projects share the layout; only what fills the fields differs.
type entryData struct {
	head     string
	detail   string
	dates    string
	location string
	summary  string
	bullets  []string
}

// entry draws one work or project item: a locked two-column head, then its
// summary and bullets.
func (p *painter) entry(e entryData) {
	d := p.d
	if strings.TrimSpace(e.head) == "" {
		e.head = e.detail
		e.detail = ""
	}

	// Keep the head with the first thing under it, so a job never starts at the
	// bottom of a page and continues alone overleaf.
	first := 0.0
	if e.summary != "" {
		first = float64(len(Wrap(e.summary, PageWidth-MarginLeft-MarginRight, d.Body, helvetica))) * d.LineHeight
	} else if len(e.bullets) > 0 {
		first = float64(len(Wrap(e.bullets[0], PageWidth-MarginLeft-MarginRight-12, d.Body, helvetica))) * d.LineHeight
	}
	p.ensure(d.LineHeight + d.BulletGap + first)

	y := p.y
	if e.detail != "" {
		p.text(MarginLeft, y, e.head, helveticaBold, d.Body, Ink)
		offset := Measure(e.head+" ", d.Body, helveticaBold)
		rest := "— " + e.detail
		if offset+Measure(rest, d.Body, helvetica) <= LeftWidth {
			p.text(MarginLeft+offset, y, rest, helvetica, d.Body, Ink)
		} else {
			p.y += d.LineHeight
			for _, line := range Wrap(rest, LeftWidth-10, d.Body, helvetica) {
				p.text(MarginLeft+10, p.y, line, helvetica, d.Body, Ink)
				p.y += d.LineHeight
			}
			p.y -= d.LineHeight
		}
	} else {
		p.text(MarginLeft, y, e.head, helveticaBold, d.Body, Ink)
	}
	p.metaBlock(y, e.dates, e.location)
	p.y += d.LineHeight

	if e.summary != "" {
		p.drawLines(MarginLeft, PageWidth-MarginLeft-MarginRight, e.summary, helveticaOblique, d.Body, Muted)
		p.y += d.BulletGap
	}
	for _, h := range e.bullets {
		p.bullet(h)
	}
	p.y += d.EntryGap
}

// projectEntry is the same grid with an entity instead of a title and the
// description standing in for the summary.
func (p *painter) projectEntry(item map[string]any) {
	name := stringField(item, "name")
	entity := stringField(item, "entity")
	roles := stringSlice(item, "roles")
	detail := entity
	if len(roles) > 0 {
		detail = joinWords(entity, strings.Join(roles, ", "))
	}
	p.entry(entryData{
		head:    name,
		detail:  detail,
		dates:   dateRange(stringField(item, "startDate"), stringField(item, "endDate"), p.o.Lang),
		summary: stringField(item, "description"),
		bullets: stringSlice(item, "highlights"),
	})
}

// metaBlock writes the right-hand column: the dates on the head's baseline and
// the location under them when the pair would not fit on one line.
func (p *painter) metaBlock(y float64, dates, location string) {
	if dates == "" && location == "" {
		return
	}
	p.meta(rightEdge, y, dates)
	if location == "" {
		return
	}
	if dates != "" && Measure(dates+" | "+location, p.d.Small, helvetica) <= MetaWidth {
		p.meta(rightEdge, y, dates+" | "+location)
		return
	}
	p.meta(rightEdge, y+p.d.Small*1.45, location)
}

// bullet draws one highlight with a hanging indent.
func (p *painter) bullet(text string) {
	d := p.d
	indent := 12.0
	lines := Wrap(text, PageWidth-MarginLeft-MarginRight-indent, d.Body, helvetica)
	for i, line := range lines {
		if p.y+d.LineHeight > p.bottom() {
			p.newPage()
		}
		if i == 0 {
			p.text(MarginLeft+2, p.y, "•", helvetica, d.Body, Muted)
		}
		p.text(MarginLeft+indent, p.y, line, helvetica, d.Body, Ink)
		p.y += d.LineHeight
	}
	p.y += d.BulletGap
}

// inline is a bold label followed by muted detail on the same line, wrapping
// under itself when the pair is too wide.
func (p *painter) inline(left, detail string) {
	d := p.d
	p.ensure(d.LineHeight * 1.5)
	y := p.y
	if detail == "" {
		p.drawLines(MarginLeft, LeftWidth, left, helveticaBold, d.Body, Ink)
		return
	}
	offset := Measure(left+" ", d.Body, helveticaBold)
	if offset+Measure(detail, d.Body, helvetica) <= PageWidth-MarginLeft-MarginRight {
		p.text(MarginLeft, y, left, helveticaBold, d.Body, Ink)
		p.text(MarginLeft+offset, y, "— "+detail, helvetica, d.Body, Ink)
		p.y += d.LineHeight
		return
	}
	p.text(MarginLeft, y, left, helveticaBold, d.Body, Ink)
	p.y += d.LineHeight
	p.drawLines(MarginLeft+10, PageWidth-MarginLeft-MarginRight-10, detail, helvetica, d.Body, Ink)
}

// twoColumn draws a locked grid row: left text, right-aligned meta at the same
// baseline.
func (p *painter) twoColumn(left, detail, meta string) {
	d := p.d
	p.ensure(d.LineHeight * 1.5)
	p.text(MarginLeft, p.y, left, helveticaBold, d.Body, Ink)
	if detail != "" {
		offset := Measure(left+" ", d.Body, helveticaBold)
		if offset+Measure(detail, d.Body, helvetica) <= LeftWidth {
			p.text(MarginLeft+offset, p.y, "— "+detail, helvetica, d.Body, Ink)
		} else {
			detailLines := Wrap(detail, LeftWidth-10, d.Body, helvetica)
			p.y += d.LineHeight
			for _, line := range detailLines {
				p.text(MarginLeft+10, p.y, line, helvetica, d.Body, Ink)
				p.y += d.LineHeight
			}
			p.y -= d.LineHeight
		}
	}
	// The meta is pinned to the head's baseline even when the detail wrapped.
	p.meta(rightEdge, p.y, meta)
	p.y += d.LineHeight
}

// finish adds the signature mark and the page footers, then trims the trailing
// gap the last block left behind.
func (p *painter) finish(doc *resume.Doc, l labels) {
	if p.o.SigSrc != "" && p.sigH > 0 {
		w := p.o.SigW
		if w == 0 {
			w = 92
		}
		y := PageHeight - MarginBottom - p.sigH
		p.add(func(c *Content) {
			c.Images = append(c.Images, ImageBox{
				Src:   p.o.SigSrc,
				Pos:   [2]float64{rightEdge - w, y},
				Width: w,
			})
		})
	}
	total := len(p.spec.Pages)
	for i := 1; i <= total; i++ {
		key := fmt.Sprint(i)
		pg := p.spec.Pages[key]
		y := PageHeight - 26
		pg.Content.Text = append(pg.Content.Text, TextBox{
			Value: Sanitize(doc.Name()),
			Pos:   [2]float64{MarginLeft, y},
			Font:  Font{Name: helvetica.Name(), Size: 7.6, Color: Muted},
		})
		pg.Content.Text = append(pg.Content.Text, TextBox{
			Value: Sanitize(fmt.Sprintf(l.Page, i, total)),
			Pos:   [2]float64{rightEdge, y},
			Width: MetaWidth,
			Align: "right",
			Font:  Font{Name: helvetica.Name(), Size: 7.6, Color: Muted},
		})
		p.spec.Pages[key] = pg
	}
}

// --- resume accessors -------------------------------------------------------

// docString is GetString with the "missing means empty" convention the compiler
// uses everywhere.
func docString(doc *resume.Doc, path string) string {
	v, _ := doc.GetString(path)
	return v
}

func stringField(obj map[string]any, key string) string {
	if key == "" {
		return ""
	}
	v, _ := obj[key].(string)
	return strings.TrimSpace(v)
}

func stringSlice(obj map[string]any, key string) []string {
	raw, _ := obj[key].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func studyType(obj map[string]any) string {
	area := stringField(obj, "area")
	kind := stringField(obj, "studyType")
	switch {
	case area == "":
		return kind
	case kind == "":
		return area
	default:
		return area + ", " + kind
	}
}

func joinWords(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, strings.TrimSpace(p))
		}
	}
	return strings.Join(kept, " — ")
}

// dateRange renders "2023-01 – today" with an en dash, which is what makes a CV
// look typeset rather than assembled. An empty end date means the role is
// current, and says so in the application language.
func dateRange(start, end, lang string) string {
	start, end = strings.TrimSpace(start), strings.TrimSpace(end)
	if start != "" && end == "" {
		if strings.HasPrefix(strings.ToLower(lang), "de") {
			return start + " – heute"
		}
		return start + " – today"
	}
	switch {
	case start == "" && end == "":
		return ""
	case start == "":
		return end
	default:
		return start + " – " + end
	}
}

// dateLabel is the letter date: the place from the resume plus today, spelled
// the way each language writes it.
func dateLabel(doc *resume.Doc, lang string) string {
	now := time.Now()
	city := docString(doc, "basics.location.city")
	if strings.HasPrefix(strings.ToLower(lang), "de") {
		date := fmt.Sprintf("%d. %s %d", now.Day(), germanMonth(now.Month()), now.Year())
		if city != "" {
			return city + ", den " + date
		}
		return date
	}
	date := now.Format("2 January 2006")
	if city != "" {
		return city + ", " + date
	}
	return date
}

// germanMonth keeps the letter date in the language of the letter: Go's
// January is not Januar.
func germanMonth(m time.Month) string {
	return [...]string{"Januar", "Februar", "März", "April", "Mai", "Juni",
		"Juli", "August", "September", "Oktober", "November", "Dezember"}[m-1]
}

type row struct{ left, detail string }

func skillRows(doc *resume.Doc) []row {
	var out []row
	for _, item := range doc.Objects("skills") {
		name := stringField(item, "name")
		var words []string
		if raw, ok := item["keywords"].([]any); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok {
					words = append(words, s)
				}
			}
		}
		out = append(out, row{left: name, detail: strings.Join(words, ", ")})
	}
	return out
}

func languageRows(doc *resume.Doc) []row {
	var out []row
	for _, item := range doc.Objects("languages") {
		out = append(out, row{
			left:   stringField(item, "language"),
			detail: stringField(item, "fluency"),
		})
	}
	return out
}
