package achievement

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/steamtest"
	"github.com/whoctl/whoctl-sdk-go/core"
	"strings"
	"testing"
)

// --- the web half ------------------------------------------------------

// --- achievements are scoped to a game ---------------------------------

// Achievements have no listing of their own, so the kind is only reachable
// through core.ScopedLister. Without it the error tells the user to run a
// command that cannot work.
func TestAchievementsAreScopedToAGame(t *testing.T) {
	h := New(steamtest.Fixture(t))
	scoped, ok := h.(core.ScopedLister)
	if !ok {
		t.Fatal("Achievement must implement core.ScopedLister, or naming a game reaches nothing")
	}

	// No API key is configured here, so the furthest a scoped listing can get
	// is the credentials error — which is exactly the proof that it resolved
	// the game and reached the Web API instead of dead-ending in a message.
	for _, app := range []string{"620", "Portal 2"} {
		_, err := scoped.ListScoped(context.Background(), app)
		if err == nil {
			t.Fatalf("%s: a scoped listing with no key must fail", app)
		}
		if !strings.Contains(err.Error(), "STEAM_API_KEY") {
			t.Errorf("%s: err = %v\nwant the credentials error, meaning the app resolved", app, err)
		}
	}
}

func TestAchievementScopeRejectsSomethingThatIsNotAGame(t *testing.T) {
	scoped := New(steamtest.Fixture(t)).(core.ScopedLister)
	_, err := scoped.ListScoped(context.Background(), "ACH_WIN_ONE_GAME")
	if err == nil {
		t.Fatal("an achievement name is not a game and must not resolve")
	}
	if !strings.Contains(err.Error(), "not an app id") {
		t.Errorf("err = %v, want it to say the argument is not an app", err)
	}
}

// Every command an error suggests has to be one the CLI can actually parse.
// The message used to name a `--app` flag that was never implemented.
func TestAchievementErrorsSuggestOnlyRealSyntax(t *testing.T) {
	h := New(steamtest.Fixture(t))
	_, listErr := h.List(context.Background())
	_, getErr := h.Get(context.Background(), "ACH_WIN_ONE_GAME")
	for _, err := range []error{listErr, getErr} {
		if err == nil {
			t.Fatal("both List and Get must explain themselves")
		}
		if strings.Contains(err.Error(), "--") {
			t.Errorf("err = %v\nwant no flag suggested: get and describe take the game as an argument", err)
		}
	}
	if !strings.Contains(listErr.Error(), "steam/achievements 620") {
		t.Errorf("List err = %v\nwant it to show the app-scoped form", listErr)
	}
}
