package launchoption

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/internal/steamtest"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
	"path/filepath"
	"testing"
)

// --- launch options ----------------------------------------------------

func TestLaunchOptionsListOnlyWhatIsSet(t *testing.T) {
	h := New(steamtest.Fixture(t))
	got := steamtest.Names(t, h)
	// The fixture has two apps in localconfig and one launch option; listing
	// both would bury the one that means something.
	if len(got) != 1 || got[0] != "620" {
		t.Fatalf("launchoptions = %v, want only the app with options set", got)
	}
}

func TestLaunchOptionApplyAndDelete(t *testing.T) {
	p, root := steamtest.Writable(t)
	h := New(p)
	ctx := context.Background()

	// An app with no options yet gets a block created for it.
	result, err := h.Apply(ctx, core.Object{
		Metadata: core.Metadata{Name: "570"},
		Spec:     &LaunchOptionSpec{Options: "gamemoderun %command%"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != core.ActionCreated {
		t.Errorf("action = %q, want created", result.Action)
	}

	if err := h.Delete(ctx, "570"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Get(ctx, "570"); !core.IsNotFound(err) {
		t.Errorf("err = %v, want not found after delete", err)
	}
	// Deleting the launch options must not delete the app's own bookkeeping.
	local, err := vdf.ReadFile(filepath.Join(root, "userdata/22202", provider.LocalConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if local.Get(append(localConfigApps, "570", "LastPlayed")...) != "1700000000" {
		t.Errorf("the app block lost data it owns")
	}
}

func TestLaunchOptionResolvesAppByName(t *testing.T) {
	obj, err := New(steamtest.Fixture(t)).Get(context.Background(), "Portal 2")
	if err != nil {
		t.Fatal(err)
	}
	if obj.Metadata.Name != "620" {
		t.Errorf("name = %q", obj.Metadata.Name)
	}
}
