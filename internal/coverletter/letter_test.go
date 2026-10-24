package coverletter

import (
	"strings"
	"testing"
)

func TestScaffoldUsesTheCollectedFacts(t *testing.T) {
	l := Scaffold(ScaffoldInput{
		Company:    "TestGmbH",
		Role:       "Platform Engineer",
		Salutation: "Sehr geehrte Damen und Herren,",
		LensName:   "AI systems",
		Points:     []string{"Built retrieval.", "Cut deploy time."},
	})
	if !strings.Contains(l.Opening, "Platform Engineer") || !strings.Contains(l.Opening, "TestGmbH") {
		t.Errorf("opening = %q", l.Opening)
	}
	if !strings.Contains(l.Opening, "AI systems") {
		t.Errorf("opening does not mention the lens: %q", l.Opening)
	}
	if len(l.Body) != 2 || l.Body[1] != "Cut deploy time." {
		t.Errorf("body = %v", l.Body)
	}
	if l.SourceLang != "en" {
		t.Errorf("source lang = %q", l.SourceLang)
	}
	if !strings.Contains(l.Closing, "TestGmbH") {
		t.Errorf("closing = %q", l.Closing)
	}
}

func TestScaffoldFallsBackRatherThanEmpty(t *testing.T) {
	l := Scaffold(ScaffoldInput{})
	if l.Opening == "" || len(l.Body) != 1 || l.Closing == "" {
		t.Fatalf("empty scaffold = %+v", l)
	}
	if l.Company != "your team" {
		t.Errorf("company fallback = %q", l.Company)
	}
}

func TestRefsUseReadablePaths(t *testing.T) {
	l := Letter{Opening: "Hi", Body: []string{"a", "b"}, Closing: "Bye"}
	refs := l.Refs()
	if len(refs) != 4 {
		t.Fatalf("refs = %+v", refs)
	}
	want := []string{"cover_letter.opening", "cover_letter.body_paragraphs[0]", "cover_letter.body_paragraphs[1]", "cover_letter.closing"}
	for i, r := range refs {
		if r.Path != want[i] {
			t.Errorf("ref %d path = %q, want %q", i, r.Path, want[i])
		}
	}
}

func TestRoleTravelsWithTheLetter(t *testing.T) {
	l := Letter{Role: "Platform Engineer", Opening: "Hi", Closing: "Bye"}
	var paths []string
	for _, r := range l.Refs() {
		paths = append(paths, r.Path)
	}
	if len(paths) != 3 || paths[0] != "cover_letter.role" {
		t.Fatalf("refs = %v", paths)
	}
	l.Apply(func(s string) (string, bool) {
		if s == "Platform Engineer" {
			return "Plattform-Ingenieur", true
		}
		return "", false
	})
	if l.Role != "Plattform-Ingenieur" {
		t.Errorf("role = %q, want the translated role", l.Role)
	}
}

func TestApplyOnlyRewritesKnownStrings(t *testing.T) {
	l := Letter{Opening: "Hello", Body: []string{"One", "Two"}, Closing: "Bye"}
	known := map[string]string{"Hello": "Hallo", "Two": "Zwei"}
	l.Apply(func(s string) (string, bool) {
		v, ok := known[s]
		return v, ok
	})
	if l.Opening != "Hallo" || l.Body[1] != "Zwei" {
		t.Errorf("letter = %+v", l)
	}
	if l.Body[0] != "One" {
		t.Errorf("unknown string should stay in the source language: %q", l.Body[0])
	}
	if l.Closing != "Bye" {
		t.Errorf("closing = %q", l.Closing)
	}
}

func TestNormalizeDropsEmptyAndDuplicateParagraphs(t *testing.T) {
	l := Letter{Opening: "  Hi  ", Body: []string{"a", " ", "a", " b "}, Closing: " Bye "}
	l.Normalize()
	if len(l.Body) != 2 || l.Body[0] != "a" || l.Body[1] != "b" {
		t.Errorf("body = %v", l.Body)
	}
	if l.Opening != "Hi" || l.Closing != "Bye" {
		t.Errorf("trim = %q / %q", l.Opening, l.Closing)
	}
}

func TestBodyTextRoundTrip(t *testing.T) {
	l := Letter{Body: []string{"first", "second"}}
	text := l.BodyText()
	if text != "first\n\nsecond" {
		t.Errorf("body text = %q", text)
	}
	var back Letter
	back.SetBodyText(text + "\r\n\r\nthird\r\n")
	if len(back.Body) != 3 || back.Body[2] != "third" {
		t.Errorf("round trip = %v", back.Body)
	}
}
