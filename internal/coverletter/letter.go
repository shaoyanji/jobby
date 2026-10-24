// Package coverletter models the Anschreiben that ships next to the CV.
//
// A letter is data (opening, paragraphs, closing) so it can be drafted in
// English, translated through the same content-addressed cache as the CV, and
// rendered by the same Typst style as the resume header.
package coverletter

import (
	"fmt"
	"strings"

	"github.com/shaoyanji/jobby/internal/resume"
)

// Letter is the body of one Anschreiben.
type Letter struct {
	Company    string   `json:"company,omitempty"`
	Role       string   `json:"role,omitempty"`
	Salutation string   `json:"salutation,omitempty"`
	Opening    string   `json:"opening"`
	Body       []string `json:"body_paragraphs"`
	Closing    string   `json:"closing"`
	// SourceLang is the language the text was written in. A de capsule whose
	// letter is English gets translated on export; a letter written in German
	// is left alone.
	SourceLang string `json:"source_lang,omitempty"`
}

// ScaffoldInput is what the wizard collects before a first draft exists.
type ScaffoldInput struct {
	Company    string
	Role       string
	Salutation string
	LensName   string
	// Points become the body paragraphs. When empty, callers should pass the
	// selected CV highlights instead of inventing prose.
	Points []string
}

// Scaffold drafts an English letter. Translation is a separate, cached step so
// the same English draft can produce a German letter without losing the source.
func Scaffold(in ScaffoldInput) Letter {
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = "the advertised role"
	}
	company := strings.TrimSpace(in.Company)
	if company == "" {
		company = "your team"
	}

	opening := fmt.Sprintf("I am applying for %s at %s.", role, company)
	if in.LensName != "" {
		opening = fmt.Sprintf(
			"I am applying for %s at %s, and the work described there sits squarely in the %s side of my background.",
			role, company, in.LensName,
		)
	}

	body := make([]string, 0, len(in.Points)+1)
	for _, p := range in.Points {
		if p = strings.TrimSpace(p); p != "" {
			body = append(body, p)
		}
	}
	if len(body) == 0 {
		body = append(body, "The attached CV lists the systems I have built and the results they produced.")
	}

	return Letter{
		Company:    company,
		Role:       role,
		Salutation: in.Salutation,
		Opening:    opening,
		Body:       body,
		Closing:    fmt.Sprintf("I would welcome the chance to discuss how that experience fits %s. Kind regards,", company),
		SourceLang: "en",
	}
}

// Refs returns the translatable strings, with stable path hints so the German
// reviewer shows "cover_letter.body[1]" rather than a hash.
func (l Letter) Refs() []resume.Ref {
	var refs []resume.Ref
	add := func(path, s string) {
		if strings.TrimSpace(s) != "" {
			refs = append(refs, resume.Ref{Path: path, EN: s})
		}
	}
	// The advertised role is prose in the letter header, so it follows the
	// application language just like the CV's basics.label.
	add("cover_letter.role", l.Role)
	add("cover_letter.opening", l.Opening)
	for i, p := range l.Body {
		add(fmt.Sprintf("cover_letter.body_paragraphs[%d]", i), p)
	}
	add("cover_letter.closing", l.Closing)
	return refs
}

// Apply rewrites every translatable string through lookup. Missing entries are
// left in the source language by the caller's lookup.
func (l *Letter) Apply(lookup func(string) (string, bool)) {
	tr := func(s string) string {
		if s == "" {
			return s
		}
		if v, ok := lookup(s); ok {
			return v
		}
		return s
	}
	l.Role = tr(l.Role)
	l.Opening = tr(l.Opening)
	l.Closing = tr(l.Closing)
	for i := range l.Body {
		l.Body[i] = tr(l.Body[i])
	}
}

// Normalize trims whitespace, drops empty paragraphs and de-duplicates the body
// so a sloppy textarea cannot produce blank pages.
func (l *Letter) Normalize() {
	clean := make([]string, 0, len(l.Body))
	seen := map[string]bool{}
	for _, p := range l.Body {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		clean = append(clean, p)
	}
	l.Body = clean
	l.Opening = strings.TrimSpace(l.Opening)
	l.Closing = strings.TrimSpace(l.Closing)
	l.Salutation = strings.TrimSpace(l.Salutation)
}

// BodyText joins paragraphs with a blank line, which is the shape a textarea
// editor wants.
func (l Letter) BodyText() string {
	return strings.Join(l.Body, "\n\n")
}

// SetBodyText splits a blank-line-separated textarea back into paragraphs.
func (l *Letter) SetBodyText(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var out []string
	for _, block := range strings.Split(text, "\n\n") {
		if b := strings.TrimSpace(block); b != "" {
			out = append(out, b)
		}
	}
	l.Body = out
}

// Payload is the object templates/cover_letter.typ reads.
type Payload struct {
	Basics map[string]any `json:"basics"`
	Letter Letter         `json:"letter"`
	Date   string         `json:"date"`
	Meta   map[string]any `json:"meta"`
}

// BuildPayload combines the CV header with the letter for the template.
func BuildPayload(basics map[string]any, l Letter, lang, date string) Payload {
	return Payload{
		Basics: basics,
		Letter: l,
		Date:   date,
		Meta:   map[string]any{"language": lang},
	}
}
