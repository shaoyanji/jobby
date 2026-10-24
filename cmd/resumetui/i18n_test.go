package main

import "testing"

func TestContentKeyIsStableAndDistinct(t *testing.T) {
	a := contentKey("hello")
	if a != contentKey("hello") {
		t.Error("same input produced different keys")
	}
	if a == contentKey("hello ") {
		t.Error("different inputs collided")
	}
	if len(a) != 64 {
		t.Errorf("key length = %d, want 64 hex chars", len(a))
	}
}

func TestLookupPrefersHumanEdit(t *testing.T) {
	c := Cache{}
	c[contentKey("Builds things")] = Entry{
		SourceEN: "Builds things",
		DEAuto:   "Baut Dinge",
		DEHuman:  "Entwickelt Systeme",
		Status:   "verified",
	}
	got, ok := c.Lookup("Builds things")
	if !ok || got != "Entwickelt Systeme" {
		t.Fatalf("Lookup = %q, %v", got, ok)
	}
}

func TestLookupFallsBackToAuto(t *testing.T) {
	c := Cache{}
	c[contentKey("x")] = Entry{SourceEN: "x", DEAuto: "y", Status: "auto"}
	if got, ok := c.Lookup("x"); !ok || got != "y" {
		t.Fatalf("Lookup = %q, %v", got, ok)
	}
}

func TestSetHumanFlipsStatus(t *testing.T) {
	c := Cache{}
	c[contentKey("en text")] = Entry{SourceEN: "en text", DEAuto: "de auto", Status: "auto"}
	c.SetHuman("en text", "de text")
	if c.Status("en text") != "verified" {
		t.Errorf("status = %s, want verified", c.Status("en text"))
	}
	c.SetHuman("en text", "")
	if c.Status("en text") != "auto" {
		t.Errorf("emptying a human edit should fall back to auto, got %s", c.Status("en text"))
	}
}

func TestCoverageCounts(t *testing.T) {
	refs := []Ref{{EN: "a"}, {EN: "b"}, {EN: "c"}}
	c := Cache{}
	c.SetHuman("a", "A")
	c[contentKey("b")] = Entry{SourceEN: "b", DEAuto: "B", Status: "auto"}
	v, a, m := c.Coverage(refs)
	if v != 1 || a != 1 || m != 1 {
		t.Errorf("coverage = %d/%d/%d, want 1/1/1", v, a, m)
	}
}
