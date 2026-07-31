package provider

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Where Steam keeps the things this provider manages. Everything is relative to
// the Steam root, and everything is KeyValues.
const (
	LibraryFoldersFile = "config/libraryfolders.vdf"
	LoginUsersFile     = "config/loginusers.vdf"
	InstallConfigFile  = "config/config.vdf"
	SteamAppsDir       = "steamapps"
	UserDataDir        = "userdata"
	// LocalConfigFile holds per-account, per-app settings.
	LocalConfigFile = "config/localconfig.vdf"
)

// steamID64Base is the offset between a 64-bit SteamID and the account id that
// names a directory under userdata. Valve's ids are a packed struct; for an
// individual account the low 32 bits are the account number.
const steamID64Base int64 = 76561197960265728

// AccountIDFromSteamID64 converts 76561198006938306 to 46672578.
func AccountIDFromSteamID64(id string) (string, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid SteamID %q", id)
	}
	if v < steamID64Base {
		// Already an account id, which is what the userdata directories are
		// named and what a user reading their own filesystem is likely to type.
		return strconv.FormatInt(v, 10), nil
	}
	return strconv.FormatInt(v-steamID64Base, 10), nil
}

// SteamID64FromAccountID is the inverse, for reporting an account's real id.
func SteamID64FromAccountID(id string) string {
	v, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil {
		return ""
	}
	if v >= steamID64Base {
		return strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v+steamID64Base, 10)
}

// SortedKeys keeps every listing in a stable order regardless of what the
// filesystem hands back.
func SortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		a, errA := strconv.ParseInt(out[i], 10, 64)
		b, errB := strconv.ParseInt(out[j], 10, 64)
		if errA == nil && errB == nil {
			return a < b
		}
		return out[i] < out[j]
	})
	return out
}

// IsNumeric reports whether a string is all digits, which is how an app id is
// told apart from a game's name.
func IsNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
