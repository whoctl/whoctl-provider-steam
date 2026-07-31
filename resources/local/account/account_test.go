package account

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/internal/steamtest"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// --- accounts ----------------------------------------------------------

func TestAccountsReadTheLoginList(t *testing.T) {
	h := New(steamtest.Fixture(t))
	got := steamtest.Names(t, h)
	if len(got) != 2 || got[0] != "gaben" {
		t.Fatalf("accounts = %v, want the newest login first", got)
	}
	obj, err := h.Get(context.Background(), "gaben")
	if err != nil {
		t.Fatal(err)
	}
	s := accountStatus(obj)
	if s.AccountID != "22202" {
		t.Errorf("accountId = %q, want the 32-bit form that names the userdata directory", s.AccountID)
	}
	if !s.MostRecent || !s.AutoLogin {
		t.Errorf("status = %+v", s)
	}
}

func TestAccountLookupAcceptsEveryNameTheUserHas(t *testing.T) {
	h := New(steamtest.Fixture(t))
	for _, name := range []string{"gaben", "Gabe", "76561197960287930", "22202"} {
		if _, err := h.Get(context.Background(), name); err != nil {
			t.Errorf("Get(%q): %v", name, err)
		}
	}
}

func TestAccountAutoLoginIsExclusive(t *testing.T) {
	p, root := steamtest.Writable(t)
	h := New(p)

	// Steam signs exactly one account in automatically. Turning it on for the
	// second has to turn it off for the first, or the client picks arbitrarily.
	_, err := h.Apply(context.Background(), core.Object{
		Metadata: core.Metadata{Name: "alt"},
		Spec:     &AccountSpec{AutoLogin: provider.BoolPtr(true)},
	})
	if err != nil {
		t.Fatal(err)
	}
	users, err := vdf.ReadFile(filepath.Join(root, provider.LoginUsersFile))
	if err != nil {
		t.Fatal(err)
	}
	if users.GetBool("76561197960287930", "AutoLogin") {
		t.Errorf("the previous account still has AutoLogin set")
	}
	if !users.GetBool("76561197960287931", "AutoLogin") {
		t.Errorf("the requested account did not get AutoLogin")
	}
}

func TestAccountSpecCarriesNoCredentials(t *testing.T) {
	obj, err := New(steamtest.Fixture(t)).Get(context.Background(), "gaben")
	if err != nil {
		t.Fatal(err)
	}
	// The same rule the linux provider applies to password hashes: an exported
	// manifest must not be able to leak anything that authenticates.
	//
	// rememberPassword is deliberately not on this list — it is a boolean about
	// whether the client keeps credentials, not a credential. What must never
	// appear are the keys that carry an actual secret, which newer clients
	// write to loginusers.vdf and local.vdf alongside the flags this kind reads.
	rendered := renderYAML(t, obj)
	for _, secret := range []string{"RefreshToken", "AccessToken", "MachineAuth", "refreshToken", "accessToken"} {
		if strings.Contains(rendered, secret) {
			t.Errorf("exported account carries %q:\n%s", secret, rendered)
		}
	}
}

func TestAccountIgnoresTokensInTheFile(t *testing.T) {
	// A newer client writes a refresh token next to the flags. Reading the
	// account must not pick it up, whatever else changes about the file.
	root := filepath.Join(t.TempDir(), "steam")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	const withToken = `"users"
{
	"76561197960287930"
	{
		"AccountName"		"gaben"
		"PersonaName"		"Gabe"
		"AutoLogin"		"1"
		"MostRecent"		"1"
		"RefreshToken"		"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.SECRET"
	}
}
`
	if err := os.WriteFile(filepath.Join(root, "config/loginusers.vdf"), []byte(withToken), 0o644); err != nil {
		t.Fatal(err)
	}
	obj, err := New(provider.New(provider.Options{SteamRoot: root})).Get(context.Background(), "gaben")
	if err != nil {
		t.Fatal(err)
	}
	if rendered := renderYAML(t, obj); strings.Contains(rendered, "SECRET") {
		t.Errorf("the token leaked into the object:\n%s", rendered)
	}
}

// --- helpers -----------------------------------------------------------

func TestAccountIDConversion(t *testing.T) {
	for in, want := range map[string]string{
		"76561197960287930": "22202",
		"22202":             "22202",
		"76561198006938306": "46672578",
	} {
		got, err := provider.AccountIDFromSteamID64(in)
		if err != nil || got != want {
			t.Errorf("provider.AccountIDFromSteamID64(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if got := provider.SteamID64FromAccountID("22202"); got != "76561197960287930" {
		t.Errorf("steamID64FromAccountID = %q", got)
	}
}

// renderYAML serializes an object the way `get -o yaml` would, so a test can
// assert on what an export actually contains.
func renderYAML(t *testing.T, obj core.Object) string {
	t.Helper()
	data, err := yaml.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// pidOfSelf is the running test process, used to make a pid file that points at
// something genuinely alive.
func pidOfSelf() string { return strconv.Itoa(os.Getpid()) }
