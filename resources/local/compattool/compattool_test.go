package compattool

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/internal/steamtest"
	"github.com/whoctl/whoctl-sdk-go/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- compatibility tools -----------------------------------------------

func TestCompatToolsSkipClearedMappings(t *testing.T) {
	h := New(steamtest.Fixture(t))
	got := steamtest.Names(t, h)
	// App 570's mapping has an empty name, which is how the client records
	// "stopped forcing a tool"; it is not a mapping.
	for _, name := range got {
		if name == "570" {
			t.Errorf("a cleared mapping was listed: %v", got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("compattools = %v, want the global default and app 620", got)
	}
}

func TestCompatToolDefaultIsAddressableByName(t *testing.T) {
	h := New(steamtest.Fixture(t))
	obj, err := h.Get(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if obj.Metadata.Name != "0" || !compatStatus(obj).Global {
		t.Errorf("obj = %+v", compatStatus(obj))
	}
}

func TestCompatToolMarksUserInstalledTools(t *testing.T) {
	obj, err := New(steamtest.Fixture(t)).Get(context.Background(), "620")
	if err != nil {
		t.Fatal(err)
	}
	// GE-Proton9-20 is a directory in the fixture's compatibilitytools.d, so it
	// is a user-installed build rather than a Valve one.
	if !compatStatus(obj).Custom {
		t.Errorf("a tool from compatibilitytools.d must be reported as custom")
	}
}

func TestCompatToolApplyKeepsUnmodelledKeys(t *testing.T) {
	p, root := steamtest.Writable(t)
	h := New(p)
	_, err := h.Apply(context.Background(), core.Object{
		Metadata: core.Metadata{Name: "570"},
		Spec:     &CompatToolSpec{Tool: "proton_9"},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, provider.InstallConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"SomethingWhoctlDoesNotModel", "DesktopUIScale"} {
		if !strings.Contains(string(data), keep) {
			t.Errorf("%q was lost from config.vdf", keep)
		}
	}
}
