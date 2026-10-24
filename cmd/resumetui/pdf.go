package main

import (
	"os/exec"
	"strings"
)

// Build runs the Taskfile pipeline (typst -> pdfcpu -> mupdf reload) so the
// post-processing rules live in one place instead of in Go.
func Build(lang string) (string, error) {
	cmd := exec.Command("task", "build", "LANG="+lang)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func Mappe(lang string) (string, error) {
	cmd := exec.Command("task", "mappe", "LANG="+lang)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Info shells out to pdfcpu for page count, version, size and attachments.
func Info(pdf string) string {
	out, err := exec.Command("pdfcpu", "info", pdf).CombinedOutput()
	if err != nil {
		return "pdfcpu info: " + strings.TrimSpace(string(out))
	}
	return strings.TrimSpace(string(out))
}

func Preview(pdf string) error {
	cmd := exec.Command("mupdf", pdf)
	return cmd.Start()
}

// Reload sends SIGHUP so mupdf re-reads the file without losing the viewport.
func Reload() {
	_ = exec.Command("pkill", "-HUP", "mupdf").Run()
}
