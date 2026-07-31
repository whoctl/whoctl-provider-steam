// Package steam is whoctl's second provider: it manages a local Steam
// installation and reads the account behind it from Steam's Web API.
//
// It is deliberately split in two halves, because the two are authenticated
// differently and only one of them can be written to.
//
//   - The local half reads and rewrites Steam's own configuration files, which
//     are all KeyValues documents. Being signed in to the client is what fills
//     them in, and nothing else is needed to manage them.
//   - The web half calls the public Web API, which does not accept the client's
//     session at all: it needs an API key from steamcommunity.com/dev/apikey.
//     Everything Valve exposes there is read-only, so those kinds serve `get`
//     and `describe` and refuse the mutating verbs rather than pretending.
//
// Package provider is the state every steam resource works from: where the
// installation is, whether the client is running, and the Web API client.
//
// It exists because each kind is its own package under resources/, and they all
// need the same handful of things.
package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/whoctl/whoctl-sdk-go/core"
	"github.com/whoctl/whoctl-sdk-go/sysexec"
)

// API group and version served by this provider.
const (
	Group   = "steam.whoctl.io"
	Version = "v1alpha1"
)

// Options configures the provider.
type Options struct {
	// Root is the filesystem root, from --root. Empty means "/". The Steam
	// directory is looked for underneath it, which is how the tests point the
	// whole provider at a fixture tree.
	Root string
	// SteamRoot names the Steam directory outright, skipping detection.
	SteamRoot string
	// APIKey and SteamID authenticate the Web API half. Both are needed; the
	// key identifies the caller and the id says whose library to read.
	APIKey  string
	SteamID string

	Runner *sysexec.Runner
}

// Provider implements core.Provider for a local Steam installation.
type Provider struct {
	Opts   Options
	Runner *sysexec.Runner

	// The Steam directory is resolved lazily so `whoctl API-resources` and the
	// documentation work on a machine with no Steam at all.
	rootOnce sync.Once
	root     string
	rootErr  error
}

// New builds the steam provider.
func New(opts Options) *Provider {
	runner := opts.Runner
	if runner == nil {
		runner = &sysexec.Runner{}
	}
	return &Provider{Opts: opts, Runner: runner}
}

// Name implements core.Provider.
func (p *Provider) Name() string { return "steam" }

// steamRoots are the standard install locations, in the order Steam itself
// prefers them. ~/.steam/steam is a symlink the client maintains, so it is the
// most reliable, and the flatpak build keeps its own tree entirely separate.
var steamRoots = []string{
	".steam/steam",
	".local/share/Steam",
	".var/app/com.valvesoftware.Steam/.local/share/Steam",
	".steam/root",
}

// SteamRoot returns the Steam directory, detecting it on first use.
func (p *Provider) SteamRoot() (string, error) {
	p.rootOnce.Do(func() { p.root, p.rootErr = p.detectRoot() })
	return p.root, p.rootErr
}

func (p *Provider) detectRoot() (string, error) {
	if p.Opts.SteamRoot != "" {
		if !isDir(p.Opts.SteamRoot) {
			return "", fmt.Errorf("no Steam installation at %s", p.Opts.SteamRoot)
		}
		return p.Opts.SteamRoot, nil
	}
	if env := os.Getenv("WHOCTL_STEAM_ROOT"); env != "" {
		if !isDir(env) {
			return "", fmt.Errorf("no Steam installation at %s (from WHOCTL_STEAM_ROOT)", env)
		}
		return env, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the home directory: %w", err)
	}
	base := p.Opts.Root
	if base == "" {
		base = "/"
	}
	// --root prefixes the home directory rather than replacing it, so a
	// fixture tree mirrors the real layout instead of inventing one.
	if base != "/" {
		home = filepath.Join(base, strings.TrimPrefix(home, "/"))
	}

	var tried []string
	for _, candidate := range steamRoots {
		Path := filepath.Join(home, candidate)
		if isDir(filepath.Join(Path, "steamapps")) || isDir(filepath.Join(Path, "config")) {
			return Path, nil
		}
		tried = append(tried, Path)
	}
	return "", fmt.Errorf("no Steam installation found: looked in %s; set WHOCTL_STEAM_ROOT to point at one", strings.Join(tried, ", "))
}

func isDir(Path string) bool {
	info, err := os.Stat(Path)
	return err == nil && info.IsDir()
}

// FirstNonEmpty is the first value somebody actually set.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ResourceType fills in the group and version shared by every kind here.
func ResourceType(t core.ResourceType) core.ResourceType {
	t.Group = Group
	t.Version = Version
	return t
}

// IsDir reports whether a path is a directory, which is how this provider
// decides whether a library is mounted and whether an installation is there.
func IsDir(path string) bool { return isDir(path) }

// BoolPtr is how an observed value becomes a spec field that round-trips: an
// absent optional field and a false one are different things.
func BoolPtr(v bool) *bool { return &v }
