// Package app manages the games installed in a Steam library.
package app

import (
	"context"
	"fmt"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
	"strconv"
	"strings"
)

// AppSpec is the desired state of an installed app.
//
// It is short because most of an appmanifest describes a download in progress —
// bytes staged, target build id, depot manifests — which is Steam's bookkeeping
// and not a state anyone declares. What is left is the two settings the client
// exposes in its own properties dialog.
type AppSpec struct {
	AutoUpdate          string `yaml:"autoUpdate,omitempty" json:"autoUpdate,omitempty" doc:"When Steam updates the app: always, onLaunch or highPriority." docExample:"always"`
	AllowOtherDownloads *bool  `yaml:"allowOtherDownloadsWhileRunning,omitempty" json:"allowOtherDownloadsWhileRunning,omitempty" doc:"Whether other downloads may continue while this app is running."`
}

// AppStatus is the observed state of an installed app.
type AppStatus struct {
	AppID       string `yaml:"appId" json:"appId" doc:"Steam's numeric id for the app, same as metadata.name." docExample:"2200"`
	Name        string `yaml:"name" json:"name" doc:"The app's display name as Steam recorded it." docExample:"Quake III Arena"`
	State       string `yaml:"state" json:"state" doc:"What the StateFlags field says: installed, updateRequired, downloading and so on." docExample:"installed"`
	StateFlags  int64  `yaml:"stateFlags" json:"stateFlags" doc:"The raw StateFlags bitmask, kept because Steam adds flags faster than they can be named." docExample:"4"`
	InstallDir  string `yaml:"installDir" json:"installDir" doc:"The directory under the library's steamapps/common." docExample:"Quake 3 Arena"`
	Library     string `yaml:"library" json:"library" doc:"The Steam library the app is installed in." docExample:"/home/user/.local/share/Steam"`
	SizeOnDisk  int64  `yaml:"sizeOnDisk" json:"sizeOnDisk" doc:"Installed size in bytes."`
	BuildID     string `yaml:"buildId,omitempty" json:"buildId,omitempty" doc:"The build currently installed." docExample:"242358"`
	LastUpdated int64  `yaml:"lastUpdated,omitempty" json:"lastUpdated,omitempty" doc:"Unix time of the last update."`
	LastPlayed  int64  `yaml:"lastPlayed,omitempty" json:"lastPlayed,omitempty" doc:"Unix time the app was last launched, or zero if never."`
	AutoUpdate  string `yaml:"autoUpdate" json:"autoUpdate" doc:"The app's update policy."`
	Manifest    string `yaml:"manifest" json:"manifest" doc:"The appmanifest file the state was read from." docExample:"/home/user/.local/share/Steam/steamapps/appmanifest_2200.acf"`
}

// autoUpdate values, as Steam's AutoUpdateBehavior numbers them.
var autoUpdateNames = map[int64]string{
	0: "always",
	1: "onLaunch",
	2: "highPriority",
}

// appStateFlags names the bits of StateFlags that a user would recognise. Steam
// has never documented these; the names come from what the client displays.
var appStateFlags = []struct {
	bit  int64
	name string
}{
	{1, "uninstalled"},
	{2, "updateRequired"},
	{4, "fullyInstalled"},
	{8, "encrypted"},
	{16, "locked"},
	{32, "filesMissing"},
	{64, "appRunning"},
	{128, "filesCorrupt"},
	{256, "updateRunning"},
	{512, "updatePaused"},
	{1024, "updateStarted"},
	{2048, "uninstalling"},
	{4096, "backupRunning"},
	{1 << 16, "reconfiguring"},
	{1 << 17, "validating"},
	{1 << 18, "addingFiles"},
	{1 << 19, "preallocating"},
	{1 << 20, "downloading"},
	{1 << 21, "staging"},
	{1 << 22, "committing"},
	{1 << 23, "updateStopping"},
}

// Handler serves the kind.
type Handler struct{ p *provider.Provider }

// New builds the handler.
func New(p *provider.Provider) core.Handler { return &Handler{p: p} }

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "App",
		Plural:      "apps",
		Singular:    "app",
		ShortNames:  []string{"game"},
		Description: "A game or tool installed in a Steam library, read from its appmanifest.",
		Columns: []core.Column{
			{Name: "APPID", Path: "metadata.name"},
			{Name: "NAME", Path: "status.name"},
			{Name: "STATE", Path: "status.state"},
			{Name: "SIZE", Path: "status.sizeOnDisk", Format: core.FormatBytes},
			{Name: "AUTOUPDATE", Wide: true, Path: "status.autoUpdate"},
			{Name: "LIBRARY", Wide: true, Path: "status.library"},
			{Name: "INSTALLDIR", Wide: true, Path: "status.installDir"},
		},
	})
}

func (h *Handler) NewSpec() any { return &AppSpec{} }

func (h *Handler) NewStatus() any { return &AppStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	manifests, err := local.AppManifests(h.p)
	if err != nil {
		return nil, err
	}
	out := make([]core.Object, 0, len(manifests))
	for _, id := range provider.SortedKeys(manifests) {
		obj, err := h.build(manifests[id])
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	ref, err := local.LookupApp(h.p, name)
	if err != nil {
		return core.Object{}, err
	}
	return h.build(ref)
}

// Apply reconciles the two settings the manifest owns. It never installs:
// installing is a download the client performs, asynchronously, over a session
// whoctl does not have — `steam steam://install/<id>` only opens a dialog for a
// human to confirm, which is not a desired state anything can converge on.
func (h *Handler) Apply(ctx context.Context, obj core.Object) (core.Result, error) {
	name := obj.Metadata.Name
	spec, ok := obj.Spec.(*AppSpec)
	if !ok || spec == nil {
		return core.Result{}, fmt.Errorf("app %q: missing or invalid spec", name)
	}
	ref, err := local.LookupApp(h.p, name)
	if err != nil {
		if core.IsNotFound(err) {
			return core.Result{}, fmt.Errorf("app %q is not installed: whoctl manages the settings of installed apps, not the downloads that install them", name)
		}
		return core.Result{}, err
	}

	root, err := vdf.ReadFile(ref.Path)
	if err != nil {
		return core.Result{}, err
	}
	if root == nil {
		return core.Result{}, fmt.Errorf("app %q: %s is empty", name, ref.Path)
	}

	var diff []string
	if spec.AutoUpdate != "" {
		want, err := autoUpdateValue(spec.AutoUpdate)
		if err != nil {
			return core.Result{}, fmt.Errorf("app %q: %w", name, err)
		}
		current, _ := root.GetInt("AutoUpdateBehavior")
		if current != want {
			root.Set(strconv.FormatInt(want, 10), "AutoUpdateBehavior")
			diff = append(diff, fmt.Sprintf("autoUpdate %s -> %s", autoUpdateName(current), spec.AutoUpdate))
		}
	}
	if spec.AllowOtherDownloads != nil {
		want := local.BoolToVDF(*spec.AllowOtherDownloads)
		if root.Get("AllowOtherDownloadsWhileRunning") != want {
			root.Set(want, "AllowOtherDownloadsWhileRunning")
			diff = append(diff, fmt.Sprintf("allowOtherDownloadsWhileRunning -> %t", *spec.AllowOtherDownloads))
		}
	}

	if len(diff) == 0 {
		obj, err := h.build(ref)
		return core.Result{Action: core.ActionUnchanged, Object: obj}, err
	}
	if err := local.CheckWritable(h.p, fmt.Sprintf("app %q", name)); err != nil {
		return core.Result{}, err
	}
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", ref.Path)) {
		return core.Result{Action: core.ActionConfigured, Object: obj, Diff: diff}, nil
	}
	if err := vdf.WriteFile(ref.Path, root); err != nil {
		return core.Result{}, err
	}
	updated, err := h.build(ref)
	return core.Result{Action: core.ActionConfigured, Object: updated, Diff: diff}, err
}

// Delete is unsupported for the same reason Apply does not install. Removing an
// appmanifest without the client's involvement leaves the game's files on disk
// and Steam's own bookkeeping out of step.
func (h *Handler) Delete(ctx context.Context, name string) error {
	if _, err := local.LookupApp(h.p, name); err != nil {
		return err
	}
	return fmt.Errorf("apps cannot be uninstalled by whoctl: deleting an appmanifest would strand the game's files and leave Steam's records inconsistent; uninstall from the client")
}

func (h *Handler) build(ref local.ManifestRef) (core.Object, error) {
	root, err := vdf.ReadFile(ref.Path)
	if err != nil {
		return core.Object{}, err
	}
	if root == nil {
		return core.Object{}, core.NotFound("app", ref.AppID)
	}

	flags, _ := root.GetInt("StateFlags")
	size, _ := root.GetInt("SizeOnDisk")
	updated, _ := root.GetInt("LastUpdated")
	played, _ := root.GetInt("LastPlayed")
	behaviour, _ := root.GetInt("AutoUpdateBehavior")
	allowOther := root.GetBool("AllowOtherDownloadsWhileRunning")

	t := h.Type()
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: ref.AppID},
		Spec: &AppSpec{
			AutoUpdate:          autoUpdateName(behaviour),
			AllowOtherDownloads: &allowOther,
		},
		Status: &AppStatus{
			AppID:       ref.AppID,
			Name:        root.Get("name"),
			State:       describeStateFlags(flags),
			StateFlags:  flags,
			InstallDir:  root.Get("installdir"),
			Library:     ref.Library,
			SizeOnDisk:  size,
			BuildID:     root.Get("buildid"),
			LastUpdated: updated,
			LastPlayed:  played,
			AutoUpdate:  autoUpdateName(behaviour),
			Manifest:    ref.Path,
		},
	}, nil
}

// describeStateFlags renders the bitmask as the names that are set. The common
// case is exactly one bit, so the common output is one word.
func describeStateFlags(flags int64) string {
	if flags == 0 {
		return "invalid"
	}
	var names []string
	for _, f := range appStateFlags {
		if flags&f.bit != 0 {
			names = append(names, f.name)
		}
	}
	if len(names) == 0 {
		return fmt.Sprintf("unknown(%d)", flags)
	}
	return strings.Join(names, ",")
}

func autoUpdateName(v int64) string {
	if name, ok := autoUpdateNames[v]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", v)
}

func autoUpdateValue(name string) (int64, error) {
	for v, n := range autoUpdateNames {
		if strings.EqualFold(n, name) {
			return v, nil
		}
	}
	return 0, fmt.Errorf("invalid autoUpdate %q: use always, onLaunch or highPriority", name)
}

func appStatus(o core.Object) *AppStatus {
	if s, ok := o.Status.(*AppStatus); ok && s != nil {
		return s
	}
	return &AppStatus{}
}
