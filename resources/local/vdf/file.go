package vdf

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadFile parses a VDF file and returns its root node. A file that is not
// there yields a nil node and no error, so a caller can tell "Steam has never
// written this" apart from "the read failed".
func ReadFile(path string) (*Node, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()

	nodes, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return Root(nodes), nil
}

// WriteFile renders a root node back over its file.
//
// The write goes through a rename so a crash cannot leave Steam with a
// half-written config — a truncated localconfig.vdf loses every launch option
// and library setting at once. Steam's own files are 0644.
func WriteFile(path string, root *Node) error {
	if root == nil {
		return fmt.Errorf("refusing to write an empty document to %s", path)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".whoctl-*.vdf")
	if err != nil {
		return err
	}
	name := tmp.Name()

	_, writeErr := tmp.WriteString(Format([]*Node{root}))
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(name)
		if writeErr != nil {
			return fmt.Errorf("writing %s: %w", path, writeErr)
		}
		return fmt.Errorf("writing %s: %w", path, closeErr)
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
