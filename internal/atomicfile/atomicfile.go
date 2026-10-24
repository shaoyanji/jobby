// Package atomicfile writes files through a temp file plus rename, so a crash
// or a full disk can never leave a half-written document behind. Everything the
// workbench persists (resume, capsule, cache, rendered JSON) goes through here.
package atomicfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path atomically.
func Write(path string, b []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

// WriteJSON marshals v with two-space indentation and a trailing newline, then
// writes it atomically.
func WriteJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return Write(path, append(b, '\n'), 0o644)
}

// WriteString is Write for text.
func WriteString(path, s string) error {
	return Write(path, []byte(s), 0o644)
}
