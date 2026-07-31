// Package steamtest is the fixture every resource package tests against.
//
// **Never touch a real Steam installation from a test.** A developer running
// this suite has a live install one directory away, so Fixture reads
// testdata/steam and nothing else, and Writable copies that tree into
// t.TempDir() before any test is allowed to write.
package steamtest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/whoctl/whoctl-sdk-go/core"

	"github.com/whoctl/whoctl-provider-steam/internal/provider"
)

// Root finds the fixture installation by walking up from the test's own
// directory, rather than counting "../".
//
// A relative path is right for one depth and silently wrong for another, and
// the failure it produces — "no Steam installation at ..." — reads like a
// missing fixture rather than a miscounted path.
func Root(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "testdata", "steam")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no testdata/steam directory above the test")
		}
		dir = parent
	}
}

// Fixture points the provider at testdata rather than at the machine's own
// Steam.
func Fixture(t *testing.T) *provider.Provider {
	t.Helper()
	return provider.New(provider.Options{SteamRoot: Root(t)})
}

// Writable copies the fixture tree into a temporary directory first, and
// returns the provider along with the root so a test can inspect what was
// written.
func Writable(t *testing.T) (*provider.Provider, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "steam")
	err := filepath.WalkDir(Root(t), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(Root(t), path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying the fixture tree: %v", err)
	}
	return provider.New(provider.Options{SteamRoot: root}), root
}

// Names lists what a handler returns, which is what most of these tests assert
// about a listing.
func Names(t *testing.T, h core.Handler) []string {
	t.Helper()
	objs, err := h.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	out := make([]string, 0, len(objs))
	for _, o := range objs {
		out = append(out, o.Metadata.Name)
	}
	return out
}
