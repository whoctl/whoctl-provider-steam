package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/whoctl/whoctl-sdk-go/core"

	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/internal/steam"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/local/launchoption"
)

// Refusing to write while the client is running is a property of the provider,
// not of any one kind: Steam holds its configuration in memory and rewrites
// these files on exit, so a change written underneath it is discarded at best.
// The assembled provider is the smallest thing that can be asked.

func writableFixture(t *testing.T) (*provider.Provider, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "steam")
	err := filepath.WalkDir(fixtureRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixtureRoot, path)
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

func TestMutationsRefuseWhileSteamIsRunning(t *testing.T) {
	p, root := writableFixture(t)
	// Steam records its pid here; pointing it at this test process is the only
	// honest way to simulate "the client is up".
	if err := os.WriteFile(filepath.Join(root, "steam.pid"), []byte(pidOfSelf()), 0o644); err != nil {
		t.Fatal(err)
	}
	if !local.Running(p) {
		t.Fatal("a live pid file must be read as a running client")
	}

	h := handlerFor(t, steam.New(steam.Options{SteamRoot: root}), "LaunchOption")
	_, err := h.Apply(context.Background(), core.Object{
		Metadata: core.Metadata{Name: "570"},
		Spec:     &launchoption.LaunchOptionSpec{Options: "gamemoderun %command%"},
	})
	if err == nil || !strings.Contains(err.Error(), "Steam is running") {
		t.Errorf("err = %v, want a refusal: the client rewrites these files on exit", err)
	}
}

func TestAStalePidFileIsNotARunningClient(t *testing.T) {
	p, root := writableFixture(t)
	// An unclean exit leaves the file behind. Trusting it would lock the user
	// out of their own configuration until they noticed and deleted it.
	if err := os.WriteFile(filepath.Join(root, "steam.pid"), []byte("999999999"), 0o644); err != nil {
		t.Fatal(err)
	}
	if local.Running(p) {
		t.Error("a pid with no process behind it must not count as running")
	}
}

// pidOfSelf is a pid that is certainly running, which is what tells a live
// client apart from a stale pid file left by an unclean exit.
func pidOfSelf() string { return strconv.Itoa(os.Getpid()) }
