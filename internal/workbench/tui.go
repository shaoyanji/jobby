package workbench

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/shaoyanji/jobby/internal/capsule"
	"github.com/shaoyanji/jobby/internal/i18n"
	"github.com/shaoyanji/jobby/internal/layout"
	"github.com/shaoyanji/jobby/internal/lens"
	"github.com/shaoyanji/jobby/internal/pdf"
	"github.com/shaoyanji/jobby/internal/resume"
	"github.com/shaoyanji/jobby/internal/tui"
)

// Session is one interactive workbench run.
type Session struct {
	Root   string
	Budget int
	Mock   bool
	// Guides draws pdfcpu's alignment grid into the next export, so the
	// blueprint can be inspected in the viewer.
	Guides  bool
	Current *capsule.Capsule
}

// Preview builds the export view for a capsule without touching the disk.
func Preview(root string, c *capsule.Capsule) (*resume.Doc, lens.Lens, error) {
	doc, err := resume.Load(filepath.Join(root, "resume.json"))
	if err != nil {
		return nil, lens.Lens{}, err
	}
	lenses, err := lens.Load(filepath.Join(root, "lenses.json"))
	if err != nil {
		return nil, lens.Lens{}, err
	}
	l, ok := lenses.Get(c.Lens)
	if !ok {
		return nil, lens.Lens{}, fmt.Errorf("unknown lens %q", c.Lens)
	}
	view := doc.Clone()
	lens.Identity(view, l)
	c.Apply(view)
	view.SetLanguage(c.Language)
	return view, l, nil
}

// Run is the menu loop. Every action returns to the menu; an error is printed
// and the loop continues, so a missing file or a failed command never drops
// the session.
func (s *Session) Run() error {
	if s.Budget == 0 {
		s.Budget = 2
	}
	for {
		action, err := s.menu()
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}
		var runErr error
		switch action {
		case "quit":
			return nil
		case "guides":
			runErr = s.toggleGuides()
		case "new":
			runErr = s.newCapsule()
		case "switch":
			runErr = s.switchCapsule()
		case "tailor":
			runErr = s.tailor()
		case "letter":
			runErr = s.editLetter()
		case "review":
			runErr = s.review()
		case "export":
			runErr = s.export()
		case "import":
			runErr = s.importPDF()
		}
		if errors.Is(runErr, huh.ErrUserAborted) {
			continue // Ctrl+C in a form returns to the menu, it does not warn
		}
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "\nerror: %v\n\n", runErr)
		}
	}
}

// toggleGuides flips the blueprint grid and, when a capsule is open, exports so
// the viewer shows the change immediately.
func (s *Session) toggleGuides() error {
	s.Guides = !s.Guides
	state := "off"
	if s.Guides {
		state = "on"
	}
	fmt.Printf("\nalignment guides: %s\n", state)
	if s.Current == nil {
		fmt.Println("open a capsule to see the grid")
		return nil
	}
	return s.export()
}

// fitLine reports the layout solver's verdict without writing anything: the
// deckbuilder shows it after every change so the page budget is never a
// surprise at export time.
func (s *Session) fitLine() string {
	if s.Current == nil {
		return ""
	}
	view, _, err := Preview(s.Root, s.Current)
	if err != nil {
		return ""
	}
	res := layout.Compile(view, layout.Options{Lang: s.Current.Language, Budget: s.Budget})
	if res.Fits {
		return fmt.Sprintf("fits on %d page(s) at %s", res.Pages, res.Density.ID)
	}
	return fmt.Sprintf("%d pages at %s — trim a bullet", res.Pages, res.Density.ID)
}

func (s *Session) title() string {
	if s.Current == nil {
		return "workbench"
	}
	return fmt.Sprintf("workbench · %s (%s, %s)", s.Current.Company, s.Current.Lens, s.Current.Language)
}

func (s *Session) menu() (string, error) {
	var choice string
	err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(s.title()).
			Options(
				huh.NewOption("New application capsule", "new"),
				huh.NewOption("Switch capsule", "switch"),
				huh.NewOption("Tailor titles & bullets", "tailor"),
				huh.NewOption("Edit cover letter (Anschreiben)", "letter"),
				huh.NewOption("Polish German translations (this application)", "review"),
				huh.NewOption("Export & stamp package", "export"),
				huh.NewOption("Toggle alignment guides (blueprint grid, then export)", "guides"),
				huh.NewOption("Import capsule from a PDF", "import"),
				huh.NewOption("Quit", "quit"),
			).
			Value(&choice),
	)).Run()
	return choice, err
}

func (s *Session) newCapsule() error {
	lenses, err := lens.Load(filepath.Join(s.Root, "lenses.json"))
	if err != nil {
		return err
	}
	if len(lenses.IDs()) == 0 {
		return errors.New("lenses.json has no lenses")
	}

	var (
		company, role, salutation, points string
		lang                              = "en"
		lensOpt                           string
	)
	opts := lenses.Options()
	lensOpt = opts[0]

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Company").Value(&company).Validate(required),
			huh.NewInput().Title("Role").Description("Leave empty to use the lens label.").Value(&role),
		),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Language").Options(huh.NewOptions("en", "de")...).Value(&lang),
			huh.NewSelect[string]().Title("Narrative lens").Options(huh.NewOptions(opts...)...).Value(&lensOpt),
		),
		huh.NewGroup(
			huh.NewInput().Title("Salutation").Description("Written in the target language.").Value(&salutation),
		),
		huh.NewGroup(
			huh.NewText().
				Title("Talking points").
				Description("One paragraph per blank-line block. Empty means: use the lens highlights.").
				Lines(8).
				Value(&points),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	id := lensIDFromOption(lensOpt, lenses)
	c, dir, err := Draft(s.Root, company, role, salutation, lang, id, paragraphs(points))
	if err != nil {
		return err
	}
	s.Current = c
	fmt.Printf("\ncreated %s\n", dir)
	return nil
}

func (s *Session) switchCapsule() error {
	sums, err := capsule.List(s.Root)
	if err != nil {
		return err
	}
	if len(sums) == 0 {
		return errors.New("no capsules yet: create one first")
	}
	labels := make([]string, 0, len(sums))
	byLabel := map[string]string{}
	for _, sum := range sums {
		label := fmt.Sprintf("%s · %s (%s)", sum.Company, sum.Role, sum.Language)
		labels = append(labels, label)
		byLabel[label] = sum.Dir
	}
	var pick string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Open capsule").Options(huh.NewOptions(labels...)...).Value(&pick),
	)).Run(); err != nil {
		return err
	}
	c, err := capsule.Load(byLabel[pick])
	if err != nil {
		return err
	}
	s.Current = c
	fmt.Printf("\nopened %s\n", c.Dir(s.Root))
	return nil
}

func (s *Session) tailor() error {
	c, err := s.requireCapsule()
	if err != nil {
		return err
	}
	doc, err := resume.Load(filepath.Join(s.Root, "resume.json"))
	if err != nil {
		return err
	}
	lenses, err := lens.Load(filepath.Join(s.Root, "lenses.json"))
	if err != nil {
		return err
	}
	l, _ := lenses.Get(c.Lens)

	work := doc.WorkItems()
	projects := doc.ProjectItems()
	titles := make([]string, len(work))
	workSel := make([][]int, len(work))
	projSel := make([][]int, len(projects))
	for i, item := range work {
		titles[i] = c.Titles[item.Key]
		if titles[i] == "" {
			titles[i] = l.Titles[item.Key]
		}
		workSel[i] = preset(c.Work[item.Key], len(item.Highlights))
	}
	for i, item := range projects {
		projSel[i] = preset(c.Projects[item.Key], len(item.Highlights))
	}

	groups := []*huh.Group{}
	for i, item := range work {
		title := item.Detail
		if title == "" {
			title = item.Key
		}
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title("Title · "+item.Key).
				Description("Empty keeps the master title ($"+title+").").
				Value(&titles[i]),
			bulletsMultiSelect(item.Key, item.Highlights, &workSel[i]),
		))
	}
	for i, item := range projects {
		groups = append(groups, huh.NewGroup(
			bulletsMultiSelect(item.Key, item.Highlights, &projSel[i]),
		))
	}
	if err := huh.NewForm(groups...).Run(); err != nil {
		return err
	}

	c.Titles = map[string]string{}
	for i, item := range work {
		if t := strings.TrimSpace(titles[i]); t != "" {
			c.Titles[item.Key] = t
		}
	}
	c.Work = map[string][]int{}
	for i, item := range work {
		c.Work[item.Key] = workSel[i]
	}
	c.Projects = map[string][]int{}
	for i, item := range projects {
		c.Projects[item.Key] = projSel[i]
	}
	if _, err := c.Save(s.Root); err != nil {
		return err
	}
	fmt.Printf("\nsaved tailoring for %s\n%s\n", c.Company, s.fitLine())
	return nil
}

func (s *Session) editLetter() error {
	c, err := s.requireCapsule()
	if err != nil {
		return err
	}
	l := c.Cover
	if l.SourceLang == "" {
		l.SourceLang = "en"
	}
	opening := l.Opening
	body := l.BodyText()
	closing := l.Closing
	salutation := l.Salutation
	source := l.SourceLang

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Salutation").Value(&salutation),
			huh.NewSelect[string]().
				Title("Letter language").
				Description("English source is translated on export; German is used as written.").
				Options(huh.NewOptions("en", "de")...).
				Value(&source),
		),
		huh.NewGroup(huh.NewText().Title("Opening").Lines(3).Value(&opening)),
		huh.NewGroup(huh.NewText().Title("Body paragraphs").Description("Separate paragraphs with a blank line.").Lines(12).Value(&body)),
		huh.NewGroup(huh.NewText().Title("Closing").Lines(3).Value(&closing)),
	)
	if err := form.Run(); err != nil {
		return err
	}

	l.Salutation = strings.TrimSpace(salutation)
	l.Opening = strings.TrimSpace(opening)
	l.Closing = strings.TrimSpace(closing)
	l.SourceLang = source
	l.SetBodyText(body)
	l.Normalize()
	c.Cover = l
	if _, err := c.Save(s.Root); err != nil {
		return err
	}
	fmt.Printf("\nsaved the letter for %s on %s\n", c.Company, filepath.Base(c.Dir(s.Root)))
	return nil
}

func (s *Session) review() error {
	c, err := s.requireCapsule()
	if err != nil {
		return err
	}
	view, _, err := Preview(s.Root, c)
	if err != nil {
		return err
	}
	cache, err := i18n.Load(filepath.Join(s.Root, "i18n_cache.json"))
	if err != nil {
		return err
	}
	refs := append(view.Refs(), c.Cover.Refs()...)
	return tui.ReviewEntries(cache, cache.UnverifiedFor(refs))
}

func (s *Session) export() error {
	c, err := s.requireCapsule()
	if err != nil {
		return err
	}
	opt := Options{
		Root:        s.Root,
		Capsule:     c,
		Budget:      s.Budget,
		Guides:      s.Guides,
		Optimize:    true,
		ReloadMupdf: true,
		Log:         func(format string, args ...any) { fmt.Printf(format+"\n", args...) },
	}
	if s.Mock {
		opt.Translator = i18n.Mock{}
	}
	rep, err := Build(opt)
	if err != nil {
		return err
	}
	for _, w := range rep.Warnings {
		fmt.Printf("warning: %s\n", w)
	}
	status := fmt.Sprintf("✔ Auto-fitted to %d pages (preset: %s)", rep.Pages, rep.Preset)
	if !rep.Fitted {
		status = fmt.Sprintf("! %d pages at the densest preset (%s)", rep.Pages, rep.Preset)
	}
	fmt.Printf("\n%s\n%s\n", status, rep.CapsuleDir)
	if s.Guides {
		fmt.Println("guides: on (pdfcpu gridlines + content box)")
	}
	if rep.Package != "" {
		fmt.Println(rep.Package)
	}
	return nil
}

func (s *Session) importPDF() error {
	var path string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("PDF to recover").
			Description("The capsule.json embedded by a previous export.").
			Value(&path).
			Validate(required),
	)).Run(); err != nil {
		return err
	}
	dir, err := ImportPDF(s.Root, path)
	if err != nil {
		return err
	}
	c, err := capsule.Load(dir)
	if err == nil {
		s.Current = c
	}
	fmt.Printf("\nrestored %s\n", dir)
	return nil
}

func (s *Session) requireCapsule() (*capsule.Capsule, error) {
	if s.Current == nil {
		return nil, errors.New("no capsule open: create or switch to one first")
	}
	return s.Current, nil
}

// ImportPDF extracts the capsule.json a previous build embedded and restores it
// under applications/. The capsule's original date is preserved, so it lands
// where it did the first time.
func ImportPDF(root, pdfPath string) (string, error) {
	tmp, err := os.MkdirTemp("", "workbench-import-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	files, err := pdf.Unpack(pdfPath, tmp, nil)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if filepath.Base(f) != "capsule.json" {
			continue
		}
		c, err := capsule.Load(f)
		if err != nil {
			return "", err
		}
		dir, err := c.Save(root)
		if err != nil {
			return "", err
		}
		c.AddBuild(capsule.BuildRecord{
			At:        time.Now().UTC().Format(time.RFC3339),
			Lang:      c.Language,
			Artifacts: []string{"imported from " + filepath.Base(pdfPath)},
		})
		if _, err := c.Save(root); err != nil {
			return "", err
		}
		return dir, nil
	}
	return "", fmt.Errorf("%s has no capsule.json attachment", filepath.Base(pdfPath))
}

func required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("required")
	}
	return nil
}

func lensIDFromOption(option string, f *lens.File) string {
	for _, id := range f.IDs() {
		if option == fmt.Sprintf("%s (%s)", f.Lenses[id].Name, id) {
			return id
		}
	}
	return option
}

func paragraphs(text string) []string {
	var out []string
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for _, block := range strings.Split(text, "\n\n") {
		if b := strings.TrimSpace(block); b != "" {
			out = append(out, b)
		}
	}
	return out
}

// preset keeps a saved selection and falls back to "all" for a new key.
func preset(saved []int, n int) []int {
	if len(saved) == 0 {
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, len(saved))
	for _, i := range saved {
		if i >= 0 && i < n {
			out = append(out, i)
		}
	}
	return out
}

func bulletsMultiSelect(title string, bullets []string, sel *[]int) *huh.MultiSelect[int] {
	options := make([]huh.Option[int], 0, len(bullets))
	for i, b := range bullets {
		options = append(options, huh.NewOption(fmt.Sprintf("%d. %s", i, clipText(b, 110)), i))
	}
	return huh.NewMultiSelect[int]().
		Title(title).
		Description("Space toggles; only listed bullets go into this application.").
		Options(options...).
		Value(sel)
}

func clipText(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
