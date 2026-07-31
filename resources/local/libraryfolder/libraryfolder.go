// Package libraryfolder manages where Steam installs games.
package libraryfolder

import (
	"context"
	"fmt"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
	"path/filepath"
	"strings"
)

// LibraryFolderSpec is the desired state of a Steam library.
//
// The path is the object's name rather than a field, because it is what
// identifies a library: two entries pointing at the same directory are the same
// library, and Steam keys them by an index that renumbers whenever one is
// removed.
type LibraryFolderSpec struct {
	Label string `yaml:"label,omitempty" json:"label,omitempty" doc:"The name the client shows for this library. Steam leaves it empty for the default one." docExample:"Games SSD"`
}

// LibraryFolderStatus is the observed state of a library.
type LibraryFolderStatus struct {
	Path      string `yaml:"path" json:"path" doc:"The library directory, same as metadata.name." docExample:"/home/user/.local/share/Steam"`
	Label     string `yaml:"label,omitempty" json:"label,omitempty" doc:"The client's name for the library."`
	Index     string `yaml:"index" json:"index" doc:"Steam's position for the library in libraryfolders.vdf." docExample:"0"`
	Apps      int    `yaml:"apps" json:"apps" doc:"How many apps Steam records as installed here."`
	Mounted   bool   `yaml:"mounted" json:"mounted" doc:"Whether the directory is reachable right now. An external drive that is unplugged is configured but not mounted."`
	Default   bool   `yaml:"default" json:"default" doc:"Whether this is the Steam installation's own library."`
	TotalSize int64  `yaml:"totalSize,omitempty" json:"totalSize,omitempty" doc:"Size of the drive as Steam last measured it, in bytes."`
}

// Handler serves the kind.
type Handler struct{ p *provider.Provider }

// New builds the handler.
func New(p *provider.Provider) core.Handler { return &Handler{p: p} }

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "LibraryFolder",
		Plural:      "libraryfolders",
		Singular:    "libraryfolder",
		ShortNames:  []string{"lib"},
		Description: "A directory Steam installs games into, from config/libraryfolders.vdf.",
		Columns: []core.Column{
			{Name: "PATH", Path: "metadata.name"},
			{Name: "LABEL", Path: "status.label"},
			{Name: "APPS", Path: "status.apps"},
			{Name: "MOUNTED", Path: "status.mounted"},
			{Name: "INDEX", Wide: true, Path: "status.index"},
			{Name: "DEFAULT", Wide: true, Path: "status.default"},
		},
	})
}

func (h *Handler) NewSpec() any { return &LibraryFolderSpec{} }

func (h *Handler) NewStatus() any { return &LibraryFolderStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	entries, err := h.entries()
	if err != nil {
		return nil, err
	}
	out := make([]core.Object, 0, len(entries))
	for _, e := range entries {
		out = append(out, h.build(e))
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	entries, err := h.entries()
	if err != nil {
		return core.Object{}, err
	}
	for _, e := range entries {
		if e.Path == name {
			return h.build(e), nil
		}
	}
	return core.Object{}, core.NotFound("libraryfolder", name)
}

// Apply adds a library or renames one.
//
// Steam builds the rest of the entry — the content id, the size tally, the app
// list — the first time it scans the directory, so a new entry carries only the
// path and label and lets the client fill in what it owns.
func (h *Handler) Apply(ctx context.Context, obj core.Object) (core.Result, error) {
	name := obj.Metadata.Name
	spec, ok := obj.Spec.(*LibraryFolderSpec)
	if !ok || spec == nil {
		return core.Result{}, fmt.Errorf("libraryfolder %q: missing or invalid spec", name)
	}
	if !filepath.IsAbs(name) {
		return core.Result{}, fmt.Errorf("libraryfolder %q: the name must be an absolute path", name)
	}

	path, err := local.Path(h.p, provider.LibraryFoldersFile)
	if err != nil {
		return core.Result{}, err
	}
	root, err := vdf.ReadFile(path)
	if err != nil {
		return core.Result{}, err
	}
	if root == nil {
		root = &vdf.Node{Key: "libraryfolders", Block: true}
	}

	var (
		existing *vdf.Node
		diff     []string
	)
	for _, entry := range root.Children {
		if entry.Get("path") == name {
			existing = entry
			break
		}
	}

	action := core.ActionConfigured
	switch {
	case existing == nil:
		if !provider.IsDir(name) {
			// A library Steam cannot see is one it will drop again on the next
			// scan, so refuse rather than write something that will not stick.
			return core.Result{}, fmt.Errorf("libraryfolder %q: the directory does not exist", name)
		}
		action = core.ActionCreated
		entry := &vdf.Node{Key: nextLibraryIndex(root), Block: true}
		entry.Set(name, "path")
		entry.Set(spec.Label, "label")
		root.Children = append(root.Children, entry)
		diff = append(diff, "added "+name)
	case existing.Get("label") != spec.Label:
		existing.Set(spec.Label, "label")
		diff = append(diff, fmt.Sprintf("label %q -> %q", existing.Get("label"), spec.Label))
	}

	if len(diff) == 0 {
		obj, err := h.Get(ctx, name)
		return core.Result{Action: core.ActionUnchanged, Object: obj}, err
	}
	if err := local.CheckWritable(h.p, "Steam libraries"); err != nil {
		return core.Result{}, err
	}
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return core.Result{Action: action, Object: obj, Diff: diff}, nil
	}
	if err := vdf.WriteFile(path, root); err != nil {
		return core.Result{}, err
	}
	updated, err := h.Get(ctx, name)
	return core.Result{Action: action, Object: updated, Diff: diff}, err
}

// Delete removes a library from Steam's list. The games in it are left where
// they are: forgetting a directory and erasing it are very different, and only
// one of them is what removing a list entry means.
func (h *Handler) Delete(ctx context.Context, name string) error {
	entries, err := h.entries()
	if err != nil {
		return err
	}
	var found *libraryEntry
	for i := range entries {
		if entries[i].Path == name {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		return core.NotFound("libraryfolder", name)
	}
	if found.Default {
		return fmt.Errorf("libraryfolder %q is the Steam installation itself and cannot be removed", name)
	}
	if err := local.CheckWritable(h.p, "Steam libraries"); err != nil {
		return err
	}

	path, err := local.Path(h.p, provider.LibraryFoldersFile)
	if err != nil {
		return err
	}
	root, err := vdf.ReadFile(path)
	if err != nil || root == nil {
		return err
	}
	kept := make([]*vdf.Node, 0, len(root.Children))
	for _, entry := range root.Children {
		if entry.Get("path") != name {
			kept = append(kept, entry)
		}
	}
	root.Children = kept
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return nil
	}
	return vdf.WriteFile(path, root)
}

type libraryEntry struct {
	Index     string
	Path      string
	Label     string
	Apps      int
	TotalSize int64
	Default   bool
}

func (h *Handler) entries() ([]libraryEntry, error) {
	steamRoot, err := h.p.SteamRoot()
	if err != nil {
		return nil, err
	}
	root, err := local.ReadVDF(h.p, provider.LibraryFoldersFile)
	if err != nil {
		return nil, err
	}
	if root == nil {
		// A client that has never opened its library settings has no file, but
		// its own directory is still a library.
		return []libraryEntry{{Index: "0", Path: steamRoot, Default: true}}, nil
	}

	var out []libraryEntry
	for _, entry := range root.Children {
		path := entry.Get("path")
		if path == "" {
			path = entry.Value
		}
		if path == "" {
			continue
		}
		size, _ := entry.GetInt("totalsize")
		apps := 0
		if node := entry.Find("apps"); node != nil {
			apps = len(node.Children)
		}
		out = append(out, libraryEntry{
			Index:     entry.Key,
			Path:      path,
			Label:     entry.Get("label"),
			Apps:      apps,
			TotalSize: size,
			Default:   sameDir(path, steamRoot),
		})
	}
	return out, nil
}

func (h *Handler) build(e libraryEntry) core.Object {
	t := h.Type()
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: e.Path},
		Spec:       &LibraryFolderSpec{Label: e.Label},
		Status: &LibraryFolderStatus{
			Path: e.Path, Label: e.Label, Index: e.Index, Apps: e.Apps,
			Mounted: provider.IsDir(e.Path), Default: e.Default, TotalSize: e.TotalSize,
		},
	}
}

// nextLibraryIndex picks a key Steam will accept. The keys are consecutive
// numbers and the client renumbers them itself, so appending after the highest
// is enough.
func nextLibraryIndex(root *vdf.Node) string {
	highest := -1
	for _, entry := range root.Children {
		if n, err := parseIndex(entry.Key); err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprint(highest + 1)
}

func parseIndex(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}

// sameDir compares two paths after resolving symlinks, because ~/.steam/steam
// is a link to ~/.local/share/Steam on most installations and the two names
// mean the same library.
func sameDir(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

func libraryStatus(o core.Object) *LibraryFolderStatus {
	if s, ok := o.Status.(*LibraryFolderStatus); ok && s != nil {
		return s
	}
	return &LibraryFolderStatus{}
}
