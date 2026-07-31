package app

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/steamtest"
	"github.com/whoctl/whoctl-sdk-go/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- apps --------------------------------------------------------------

func TestAppsAreReadFromEveryLibrary(t *testing.T) {
	h := New(steamtest.Fixture(t))
	got := steamtest.Names(t, h)
	// 440 is recorded in a library that does not exist in the fixture, which is
	// what an unplugged external drive looks like: configured, not readable.
	if len(got) != 2 {
		t.Fatalf("apps = %v, want the two whose manifests are present", got)
	}

	obj, err := h.Get(context.Background(), "620")
	if err != nil {
		t.Fatal(err)
	}
	status := appStatus(obj)
	if status.Name != "Portal 2" || status.State != "fullyInstalled" {
		t.Errorf("status = %+v", status)
	}
	if status.SizeOnDisk != 9000000 {
		t.Errorf("sizeOnDisk = %d", status.SizeOnDisk)
	}
}

func TestAppStateFlagsAreDecomposed(t *testing.T) {
	// 1026 is updateRequired|updateStarted, and reading it as one opaque number
	// is exactly what the STATE column exists to avoid.
	obj, err := New(steamtest.Fixture(t)).Get(context.Background(), "570")
	if err != nil {
		t.Fatal(err)
	}
	if got := appStatus(obj).State; got != "updateRequired,updateStarted" {
		t.Errorf("state = %q", got)
	}
}

func TestAppResolvesByName(t *testing.T) {
	obj, err := New(steamtest.Fixture(t)).Get(context.Background(), "Portal 2")
	if err != nil {
		t.Fatalf("an installed app must resolve by its display name: %v", err)
	}
	if obj.Metadata.Name != "620" {
		t.Errorf("name = %q, want the app id", obj.Metadata.Name)
	}
}

func TestAppApplyChangesOnlyWhatItModels(t *testing.T) {
	p, root := steamtest.Writable(t)
	h := New(p)

	obj, err := h.Get(context.Background(), "620")
	if err != nil {
		t.Fatal(err)
	}
	obj.Spec = &AppSpec{AutoUpdate: "onLaunch"}
	result, err := h.Apply(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != core.ActionConfigured {
		t.Errorf("action = %q", result.Action)
	}

	manifest := filepath.Join(root, "steamapps/appmanifest_620.acf")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"AutoUpdateBehavior"		"1"`) {
		t.Errorf("the change was not written:\n%s", data)
	}
	// The depot block is Steam's bookkeeping and has to survive a rewrite.
	if !strings.Contains(string(data), "InstalledDepots") || !strings.Contains(string(data), "8575178398231350170") {
		t.Errorf("unmodelled keys were lost:\n%s", data)
	}
}

func TestAppApplyIsIdempotent(t *testing.T) {
	p, _ := steamtest.Writable(t)
	h := New(p)
	obj, err := h.Get(context.Background(), "620")
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.Apply(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != core.ActionUnchanged {
		t.Errorf("re-applying what get returned must be unchanged, got %q", result.Action)
	}
}

func TestAppRefusesToUninstall(t *testing.T) {
	err := New(steamtest.Fixture(t)).Delete(context.Background(), "620")
	if err == nil || !strings.Contains(err.Error(), "cannot be uninstalled") {
		t.Errorf("err = %v, want a refusal that explains itself", err)
	}
}

// --- the running-client guard ------------------------------------------
