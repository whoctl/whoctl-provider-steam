// Package local is what the kinds reading Steam's own files share: where the
// libraries and the app manifests are, how an app id is resolved from a name,
// and the guard that refuses to write while the client is running.
//
// It sits at the root of the family because nothing but these kinds reads a
// KeyValues file — with two deliberate exceptions in resources/web, where a
// game's install state and an achievement's game come from here.
package local

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/whoctl/whoctl-sdk-go/core"

	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
)

// AppManifests returns every appmanifest_*.acf across every library, keyed by
// app id, along with the library each was found in.
func AppManifests(p *provider.Provider) (map[string]ManifestRef, error) {
	libraries, err := LibraryPaths(p)
	if err != nil {
		return nil, err
	}
	out := map[string]ManifestRef{}
	for _, library := range libraries {
		dir := filepath.Join(library, provider.SteamAppsDir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			// A library recorded in libraryfolders.vdf but not currently
			// mounted is normal for an external drive, and not a read failure.
			continue
		}
		for _, e := range entries {
			appID, ok := AppIDFromManifest(e.Name())
			if !ok {
				continue
			}
			out[appID] = ManifestRef{
				AppID:   appID,
				Path:    filepath.Join(dir, e.Name()),
				Library: library,
			}
		}
	}
	return out, nil
}

// LibraryPaths returns every Steam library on the machine, the main one first.
//
// Games live in whichever library the user installed them into, so anything
// that walks appmanifests has to walk all of them; a listing that only covered
// the default library would silently omit whatever is on the second disk.
func LibraryPaths(p *provider.Provider) ([]string, error) {
	root, err := p.SteamRoot()
	if err != nil {
		return nil, err
	}
	out := []string{root}
	seen := map[string]bool{root: true}

	folders, err := ReadVDF(p, provider.LibraryFoldersFile)
	if err != nil {
		return nil, err
	}
	if folders == nil {
		return out, nil
	}
	for _, entry := range folders.Children {
		Path := entry.Get("Path")
		if Path == "" {
			// Very old clients wrote "1" "/Path" instead of a block.
			Path = entry.Value
		}
		if Path == "" || seen[Path] {
			continue
		}
		seen[Path] = true
		out = append(out, Path)
	}
	return out, nil
}

// LookupApp finds an installed app by id or by display name.
func LookupApp(p *provider.Provider, name string) (ManifestRef, error) {
	manifests, err := AppManifests(p)
	if err != nil {
		return ManifestRef{}, err
	}
	if ref, ok := manifests[strings.TrimSpace(name)]; ok {
		return ref, nil
	}
	// A game is far easier to name than to number, so a display name resolves
	// too when it is unambiguous.
	var matches []ManifestRef
	for _, id := range provider.SortedKeys(manifests) {
		ref := manifests[id]
		root, err := vdf.ReadFile(ref.Path)
		if err != nil || root == nil {
			continue
		}
		if strings.EqualFold(root.Get("name"), name) {
			matches = append(matches, ref)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return ManifestRef{}, core.NotFound("app", name)
	default:
		return ManifestRef{}, fmt.Errorf("app %q is ambiguous: %d installed apps share that name, use the app id", name, len(matches))
	}
}

// appNames maps app ids to display names for the APP column. Only installed
// apps have one; options can be set for anything the account owns.
// AppNames maps app ids to display names, for the columns that show one.
func AppNames(p *provider.Provider) (map[string]string, error) {
	manifests, err := AppManifests(p)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(manifests))
	for id, ref := range manifests {
		if root, err := vdf.ReadFile(ref.Path); err == nil && root != nil {
			out[id] = root.Get("name")
		}
	}
	return out, nil
}

// resolveAppID accepts an app id or an installed game's name. An id is taken at
// face value rather than checked against the manifests, because the kinds that
// ask about a game — launch options, achievements — are meaningful for anything
// the account owns, not only for what happens to be installed right now.
// ResolveAppID accepts an app id or an installed game's name.
func ResolveAppID(p *provider.Provider, name string) (string, error) {
	name = strings.TrimSpace(name)
	if provider.IsNumeric(name) {
		return name, nil
	}
	ref, err := LookupApp(p, name)
	if err != nil {
		return "", fmt.Errorf("%q is not an app id, and no installed app is named that", name)
	}
	return ref.AppID, nil
}

// configPath resolves the localconfig.vdf of the named account, defaulting to
// the one the client used last.
// ConfigPath resolves the localconfig.vdf of the named account.
func ConfigPath(p *provider.Provider, account string) (string, string, error) {
	steamID, name, err := ResolveAccount(p, account)
	if err != nil {
		return "", "", err
	}
	path, err := UserDataPath(p, steamID, provider.LocalConfigFile)
	return path, name, err
}

// UserDataPath resolves a file under the acting account's userdata directory.
//
// An empty steamID means "whoever last signed in", which is the only sensible
// default: these files are per account, and a machine almost always has one
// account that matters.
func UserDataPath(p *provider.Provider, steamID string, rel ...string) (string, error) {
	if steamID == "" {
		var err error
		steamID, err = MostRecentSteamID(p)
		if err != nil {
			return "", err
		}
	}
	accountID, err := provider.AccountIDFromSteamID64(steamID)
	if err != nil {
		return "", err
	}
	return Path(p, append([]string{provider.UserDataDir, accountID}, rel...)...)
}

// resolveAccount turns a spec's account field into a SteamID and a display
// name, defaulting to the account the client used last.
func ResolveAccount(p *provider.Provider, account string) (string, string, error) {
	users, err := LoginUsers(p)
	if err != nil {
		return "", "", err
	}
	if account == "" {
		id, err := MostRecentSteamID(p)
		if err != nil {
			return "", "", err
		}
		for _, u := range users {
			if u.SteamID64 == id {
				return id, u.AccountName, nil
			}
		}
		return id, "", nil
	}
	accountID, _ := provider.AccountIDFromSteamID64(account)
	for _, u := range users {
		id, _ := provider.AccountIDFromSteamID64(u.SteamID64)
		if u.AccountName == account || u.PersonaName == account || u.SteamID64 == account || (accountID != "" && id == accountID) {
			return u.SteamID64, u.AccountName, nil
		}
	}
	return "", "", fmt.Errorf("no account %q has signed in on this machine", account)
}

// ensureBlock walks a path creating blocks, and returns the last one.
// EnsureBlock creates the blocks along a KeyValues path.
func EnsureBlock(root *vdf.Node, path ...string) *vdf.Node {
	// Set followed by Delete is the cheapest way to reach vdf's own block
	// creation without exporting it: the placeholder makes the parents, and
	// removing it leaves them behind.
	root.Set("", append(append([]string{}, path...), "__whoctl")...)
	root.Delete(append(append([]string{}, path...), "__whoctl")...)
	return root.Find(path...)
}

func AppIDFromManifest(name string) (string, bool) {
	if !strings.HasPrefix(name, "appmanifest_") || !strings.HasSuffix(name, ".acf") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, "appmanifest_"), ".acf")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return "", false
	}
	return id, true
}

type ManifestRef struct {
	AppID   string
	Path    string
	Library string
}

// ErrClientRunning explains the refusal, including how to get past it.
func CheckWritable(p *provider.Provider, what string) error {
	if Running(p) {
		return core.Refusedf("Steam is running: quit the client before changing %s, or it will overwrite the change on exit", what)
	}
	return nil
}

// Running reports whether a Steam client is running, and is the guard on every
// local mutation.
//
// Steam keeps its configuration in memory and rewrites these files on exit, so
// a change written underneath a running client is silently discarded at best
// and interleaved with the client's own write at worst. Refusing is the only
// honest option: there is no lock to take and no way to ask the client to
// reload.
func Running(p *provider.Provider) bool {
	root, err := p.SteamRoot()
	if err != nil {
		return false
	}
	// Steam writes its pid here and removes the file on a clean exit.
	pidFile := filepath.Join(root, "steam.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	// A stale pid file outlives an unclean exit, so the process has to actually
	// be there. Getting this wrong in the safe direction is not an option: a
	// false "not running" lets a mutation through while the client is up, and
	// Steam then discards it on exit.
	return processExists(pid)
}

// BoolToVDF is how a boolean is spelled in a KeyValues file: Steam writes "1"
// and "0", and a file saying "true" is a file the client ignores.
func BoolToVDF(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func Path(p *provider.Provider, rel ...string) (string, error) {
	root, err := p.SteamRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{root}, rel...)...), nil
}

// ReadVDF parses one of Steam's files. A file that has never been written
// yields a nil node, which every caller reads as "nothing configured".
func ReadVDF(p *provider.Provider, rel ...string) (*vdf.Node, error) {
	Path, err := Path(p, rel...)
	if err != nil {
		return nil, err
	}
	return vdf.ReadFile(Path)
}

// LoginUsers returns the accounts the client has signed in, newest first.
func LoginUsers(p *provider.Provider) ([]LoginUser, error) {
	root, err := ReadVDF(p, provider.LoginUsersFile)
	if err != nil || root == nil {
		return nil, err
	}
	var out []LoginUser
	for _, node := range root.Children {
		ts, _ := node.GetInt("Timestamp")
		out = append(out, LoginUser{
			SteamID64:        node.Key,
			AccountName:      node.Get("AccountName"),
			PersonaName:      node.Get("PersonaName"),
			AutoLogin:        node.GetBool("AutoLogin"),
			RememberPassword: node.GetBool("RememberPassword"),
			WantsOfflineMode: node.GetBool("WantsOfflineMode"),
			MostRecent:       node.GetBool("MostRecent"),
			Timestamp:        ts,
		})
	}
	// Steam only sometimes writes MostRecent, but it always writes Timestamp,
	// so the newest login is the dependable answer to "who is this machine's
	// user".
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out, nil
}

// MostRecentSteamID is the account the per-user kinds act on by default, and
// the one the Web API half reads when no id was configured.
func MostRecentSteamID(p *provider.Provider) (string, error) {
	users, err := LoginUsers(p)
	if err != nil {
		return "", err
	}
	for _, u := range users {
		if u.MostRecent {
			return u.SteamID64, nil
		}
	}
	if len(users) > 0 {
		return users[0].SteamID64, nil
	}
	return "", fmt.Errorf("no Steam account has signed in on this machine")
}

type LoginUser struct {
	SteamID64        string
	AccountName      string
	PersonaName      string
	AutoLogin        bool
	RememberPassword bool
	WantsOfflineMode bool
	MostRecent       bool
	Timestamp        int64
}
