package libraryfolder

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/steamtest"
	"github.com/whoctl/whoctl-sdk-go/core"
	"strings"
	"testing"
)

// --- libraries ---------------------------------------------------------

func TestLibraryFoldersReportWhatIsMounted(t *testing.T) {
	h := New(steamtest.Fixture(t))
	objs, err := h.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("got %d libraries, want 2", len(objs))
	}
	for _, o := range objs {
		if libraryStatus(o).Mounted {
			t.Errorf("%q is not a real directory and must not read as mounted", o.Metadata.Name)
		}
	}
	if libraryStatus(objs[1]).Label != "Games SSD" {
		t.Errorf("label = %q", libraryStatus(objs[1]).Label)
	}
}

func TestLibraryFolderRefusesADirectoryThatIsNotThere(t *testing.T) {
	p, _ := steamtest.Writable(t)
	h := New(p)
	_, err := h.Apply(context.Background(), core.Object{
		Metadata: core.Metadata{Name: "/nowhere/at/all"},
		Spec:     &LibraryFolderSpec{},
	})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("err = %v, want a refusal: Steam drops a library it cannot see", err)
	}
}
