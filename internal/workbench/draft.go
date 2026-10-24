package workbench

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shaoyanji/jobby/internal/capsule"
	"github.com/shaoyanji/jobby/internal/coverletter"
	"github.com/shaoyanji/jobby/internal/lens"
	"github.com/shaoyanji/jobby/internal/resume"
)

// Draft creates and saves a new capsule for a company, seeded from a lens. It is
// the single place that turns "company + lens + a few answers" into an
// application folder, shared by `workbench new` and the TUI wizard.
func Draft(root, company, role, salutation, lang, lensID string, points []string) (*capsule.Capsule, string, error) {
	if strings.TrimSpace(company) == "" {
		return nil, "", fmt.Errorf("a company name is required")
	}
	doc, err := resume.Load(filepath.Join(root, "resume.json"))
	if err != nil {
		return nil, "", err
	}
	lenses, err := lens.Load(filepath.Join(root, "lenses.json"))
	if err != nil {
		return nil, "", err
	}
	l, ok := lenses.Get(lensID)
	if !ok {
		return nil, "", fmt.Errorf("unknown lens %q (have: %s)", lensID, strings.Join(lenses.IDs(), ", "))
	}
	if lang == "" {
		lang = "en"
	}
	if strings.TrimSpace(role) == "" {
		// "the advertised role" reads like a placeholder in a letter; the lens
		// label is an honest description of what is being applied for.
		role = l.Label
	}
	if salutation == "" {
		salutation = defaultSalutation(lang)
	}

	c := capsule.New(company)
	c.Role = role
	c.Salutation = salutation
	c.Language = lang
	c.Lens = l.ID
	c.Label = l.Label
	c.Titles = copyStringMap(l.Titles)
	c.Work, c.Projects = l.Selection(doc)
	c.FileStem = doc.MetaString("file_stem")
	if c.FileStem == "" {
		c.FileStem = capsule.DefaultFileStem(doc.Name())
	}

	view := doc.Clone()
	lens.Identity(view, l)
	c.Apply(view)
	seed := points
	if len(seed) == 0 {
		seed = seedPoints(view)
	}
	c.Cover = coverletter.Scaffold(coverletter.ScaffoldInput{
		Company:    company,
		Role:       role,
		Salutation: salutation,
		LensName:   l.Name,
		Points:     seed,
	})
	c.Cover.Normalize()

	dir, err := c.Save(root)
	if err != nil {
		return nil, "", err
	}
	return c, dir, nil
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
