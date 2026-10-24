package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaoyanji/jobby/internal/resume"
)

func docFrom(t *testing.T, body string) *resume.Doc {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resume.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := resume.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func emptyCache(t *testing.T) *Cache {
	t.Helper()
	c, err := Load(filepath.Join(t.TempDir(), "i18n_cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHashIsTrimmed(t *testing.T) {
	if Hash("  hello ") != Hash("hello") {
		t.Error("hash must ignore surrounding whitespace")
	}
	if len(Hash("hello")) != 12 {
		t.Errorf("hash length = %d, want 12", len(Hash("hello")))
	}
	if Hash("a") == Hash("b") {
		t.Error("distinct strings must have distinct hashes")
	}
}

func TestLookupPrefersOverride(t *testing.T) {
	c := emptyCache(t)
	c.SetAuto("Builds things", "Baut Dinge", "basics.summary")
	if got, ok := c.Lookup("Builds things"); !ok || got != "Baut Dinge" {
		t.Fatalf("auto lookup = %q, %v", got, ok)
	}
	c.SetOverride("Builds things", "Baut Sachen", true)
	if got, ok := c.Lookup("Builds things"); !ok || got != "Baut Sachen" {
		t.Fatalf("override lookup = %q, %v", got, ok)
	}
	if s := c.StatusOf("Builds things"); s != StatusVerified {
		t.Errorf("status = %q", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "i18n_cache.json")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c.SetAuto("One", "Eins", "basics.label")
	c.SetOverride("Two", "Zwei", true)
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []Entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("cache is not a flat array: %v\n%s", err, raw)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d", len(entries))
	}

	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := again.Lookup("One"); got != "Eins" {
		t.Errorf("reloaded auto = %q", got)
	}
	if s := again.StatusOf("Two"); s != StatusVerified {
		t.Errorf("reloaded status = %q", s)
	}
}

func TestLegacyMapIsMigrated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "i18n_cache.json")
	legacy := `{
	  "` + Hash("Hello") + `": {"source_en": "Hello", "de_auto": "Hallo", "status": "auto"},
	  "` + Hash("Bye") + `": {"source_en": "Bye", "de_auto": "Tschüss", "de_human": "Bis bald", "status": "verified"}
	}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Lookup("Hello"); got != "Hallo" {
		t.Errorf("Hello = %q", got)
	}
	if got, _ := c.Lookup("Bye"); got != "Bis bald" {
		t.Errorf("Bye = %q", got)
	}
	if s := c.StatusOf("Bye"); s != StatusVerified {
		t.Errorf("Bye status = %q", s)
	}
}

func TestSyncFillsMissingOnly(t *testing.T) {
	doc := docFrom(t, `{"basics":{"label":"Engineer","summary":"Builds systems"},"work":[{"name":"A","highlights":["Did a thing"]}]}`)
	c := emptyCache(t)
	c.SetOverride("Engineer", "Ingenieur", true)

	saved := 0
	stats, err := c.Sync(doc.Refs(), SyncOptions{
		Translator: Mock{},
		Save: func() error {
			saved++
			return nil
		},
		Delay: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Lookup("Engineer"); got != "Ingenieur" {
		t.Errorf("override was overwritten: %q", got)
	}
	if got, ok := c.Lookup("Builds systems"); !ok || !strings.HasPrefix(got, "[de] ") {
		t.Errorf("auto = %q, %v", got, ok)
	}
	if saved != 2 {
		t.Errorf("saved %d times, want 2 (one per new string)", saved)
	}
	if stats.Total != 3 || stats.Verified != 1 || stats.Auto != 2 {
		t.Errorf("stats = %+v", stats)
	}

	// A second pass has nothing to do.
	again, err := c.Sync(doc.Refs(), SyncOptions{Translator: Mock{}, Delay: 0})
	if err != nil {
		t.Fatal(err)
	}
	if again.Missing != 0 || again.Auto != 2 {
		t.Errorf("second sync stats = %+v", again)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	doc := docFrom(t, `{"basics":{"label":"Engineer"}}`)
	c := emptyCache(t)
	stats, err := c.Sync(doc.Refs(), SyncOptions{Translator: Mock{}, DryRun: true, Delay: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.EntriesView()) != 0 {
		t.Fatalf("dry run wrote %d entries", len(c.EntriesView()))
	}
	if stats.Missing != 1 {
		t.Errorf("missing = %d", stats.Missing)
	}
}

func TestSyncReportsPerItemFailures(t *testing.T) {
	doc := docFrom(t, `{"basics":{"label":"Engineer"}}`)
	c := emptyCache(t)
	_, err := c.Sync(doc.Refs(), SyncOptions{Translator: failing{}, Delay: 0})
	if err == nil {
		t.Fatal("expected an error from a failing translator")
	}
	if !strings.Contains(err.Error(), "basics.label") {
		t.Errorf("error should name the path: %v", err)
	}
}

type failing struct{}

func (failing) Name() string                     { return "failing" }
func (failing) Translate(string) (string, error) { return "", os.ErrDeadlineExceeded }

func TestMarkOrphans(t *testing.T) {
	doc := docFrom(t, `{"basics":{"label":"Engineer"}}`)
	c := emptyCache(t)
	c.SetAuto("Engineer", "Ingenieur", "basics.label")
	c.SetAuto("Old bullet", "Alter Punkt", "work[0].highlights[0]")

	if n := c.MarkOrphans(doc.Refs()); n != 1 {
		t.Errorf("newly orphaned = %d, want 1", n)
	}
	stats := c.StatsFor(doc.Refs())
	if stats.Orphaned != 1 {
		t.Errorf("stats.Orphaned = %d", stats.Orphaned)
	}
	for _, e := range c.EntriesView() {
		if e.EN == "Old bullet" && !e.Orphaned {
			t.Error("old bullet not marked orphaned")
		}
		if e.EN == "Engineer" && e.Orphaned {
			t.Error("live bullet marked orphaned")
		}
	}
}

func TestApplyTranslatesDocument(t *testing.T) {
	doc := docFrom(t, `{"basics":{"label":"Engineer","summary":"Untranslated"},"work":[{"name":"A","position":"Engineer","highlights":["Did it"]}],"meta":{"language":"en","keep":"me"}}`)
	c := emptyCache(t)
	c.SetAuto("Engineer", "Ingenieur", "basics.label")

	translated, missing := Apply(doc, c, "de")
	if translated != 2 {
		t.Errorf("translated = %d, want 2 (label + position)", translated)
	}
	if missing != 2 {
		t.Errorf("missing = %d, want 2 (summary + highlight)", missing)
	}
	if got := doc.PrimaryLabel(); got != "Ingenieur" {
		t.Errorf("label = %q", got)
	}
	if got := doc.Language(); got != "de" {
		t.Errorf("language = %q", got)
	}
	if doc.MetaString("keep") != "me" {
		t.Error("unrelated meta key was dropped")
	}
	if doc.Strings("work[0].highlights")[0] != "Did it" {
		t.Error("missing translation must fall back to English, not empty text")
	}
}

func TestApplyEnglishIsANoop(t *testing.T) {
	doc := docFrom(t, `{"basics":{"label":"Engineer"}}`)
	c := emptyCache(t)
	c.SetAuto("Engineer", "Ingenieur", "basics.label")
	translated, missing := Apply(doc, c, "en")
	if translated != 0 || missing != 0 {
		t.Errorf("en apply = %d/%d", translated, missing)
	}
	if doc.PrimaryLabel() != "Engineer" {
		t.Error("en apply translated the document")
	}
}

func TestGlossaryLongestFirstAndCaseInsensitive(t *testing.T) {
	g := Glossary{
		"eval harnesses": "Evaluierungs-Harnesses",
		"harness":        "Harness",
		"MCP":            "MCP",
	}
	got := g.Apply("Die EVAL HARNESSES nutzen MCP und ein Harness.")
	want := "Die Evaluierungs-Harnesses nutzen MCP und ein Harness."
	if got != want {
		t.Errorf("glossary = %q, want %q", got, want)
	}
}

func TestUnverifiedFilter(t *testing.T) {
	c := emptyCache(t)
	c.SetAuto("a", "A", "work[0].highlights[0]")
	c.SetAuto("b", "B", "basics.summary")
	c.SetOverride("c", "C", true)

	all := c.Unverified("")
	if len(all) != 2 {
		t.Fatalf("unverified = %d, want 2", len(all))
	}
	work := c.Unverified("work")
	if len(work) != 1 || work[0].PathHint != "work[0].highlights[0]" {
		t.Fatalf("work filter = %+v", work)
	}
}

// A glossary entry for the whole string pins it: "Go" is a programming
// language, and a machine translator that turns it into "Gehen" has to be
// overruled without spending a call.
func TestGlossaryExactTermSkipsTheTranslator(t *testing.T) {
	calls := 0
	tr := countingTranslator{calls: &calls}
	c, err := Load(filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	refs := []resume.Ref{{Path: "skills[0].name", EN: "Go"}, {Path: "skills[1].name", EN: "Distributed Systems"}}
	stats, err := c.Sync(refs, SyncOptions{
		Translator: tr,
		Glossary:   Glossary{"Go": "Go"},
		DryRun:     false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("translator calls = %d, want only the non-pinned string", calls)
	}
	de, ok := c.Lookup("Go")
	if !ok || de != "Go" {
		t.Errorf("Go -> %q", de)
	}
	if stats.Auto != 2 {
		t.Errorf("stats = %+v", stats)
	}
}

type countingTranslator struct{ calls *int }

func (c countingTranslator) Translate(en string) (string, error) {
	*c.calls = *c.calls + 1
	return "DE " + en, nil
}

func (c countingTranslator) Name() string { return "counting" }
