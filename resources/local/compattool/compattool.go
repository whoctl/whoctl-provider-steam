// Package compattool manages which Proton runs which game.
package compattool

import (
	"context"
	"fmt"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
	"os"
	"path/filepath"
	"strings"
)

// compatToolMapping is where the client records which Proton build runs which
// app. The key "0" is the global default applied to everything without an entry
// of its own.
var compatToolMapping = []string{"Software", "Valve", "Steam", "CompatToolMapping"}

const globalCompatKey = "0"

// CompatToolSpec is the desired compatibility tool for one app.
//
// The object's name is the app id, or the word "default" for the machine-wide
// setting. Forcing a tool is what the client's "Force the use of a specific
// Steam Play compatibility tool" checkbox writes.
type CompatToolSpec struct {
	Tool     string `yaml:"tool" json:"tool" doc:"The tool to run the app under, named as Steam names it: a built-in build such as proton_experimental, or the directory name of something in compatibilitytools.d." docFlags:"required" docExample:"proton_experimental"`
	Config   string `yaml:"config,omitempty" json:"config,omitempty" doc:"Steam's extra config word for the mapping, which the client writes as it sees fit." docExample:"noesync"`
	Priority string `yaml:"priority,omitempty" json:"priority,omitempty" doc:"Steam's priority for the mapping. Left alone when omitted." docExample:"250"`
}

// CompatToolStatus is the observed state.
type CompatToolStatus struct {
	AppID    string `yaml:"appId" json:"appId" doc:"The app the mapping applies to, or 0 for the machine-wide default." docExample:"2200"`
	AppName  string `yaml:"appName,omitempty" json:"appName,omitempty" doc:"The app's display name, when it is installed."`
	Tool     string `yaml:"tool" json:"tool" doc:"The compatibility tool in force." docExample:"proton_experimental"`
	Config   string `yaml:"config,omitempty" json:"config,omitempty" doc:"Steam's extra config word."`
	Priority string `yaml:"priority,omitempty" json:"priority,omitempty" doc:"Steam's priority for the mapping."`
	Global   bool   `yaml:"global" json:"global" doc:"Whether this is the machine-wide default rather than a per-app override."`
	Custom   bool   `yaml:"custom" json:"custom" doc:"Whether the tool is a user-installed one under compatibilitytools.d rather than a Valve build."`
	File     string `yaml:"file" json:"file" doc:"The config.vdf the mapping was read from."`
}

// Handler serves the kind.
type Handler struct{ p *provider.Provider }

// New builds the handler.
func New(p *provider.Provider) core.Handler { return &Handler{p: p} }

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "CompatTool",
		Plural:      "compattools",
		Singular:    "compattool",
		ShortNames:  []string{"proton"},
		Description: "Which Proton or other compatibility tool runs an app, from config/config.vdf.",
		Columns: []core.Column{
			{Name: "APPID", Path: "metadata.name"},
			{Name: "APP", Path: "status.appName"},
			{Name: "TOOL", Path: "status.tool"},
			{Name: "CUSTOM", Path: "status.custom"},
			{Name: "PRIORITY", Wide: true, Path: "status.priority"},
			{Name: "CONFIG", Wide: true, Path: "status.config"},
		},
	})
}

func (h *Handler) NewSpec() any { return &CompatToolSpec{} }

func (h *Handler) NewStatus() any { return &CompatToolStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	mapping, _, err := h.mapping()
	if err != nil || mapping == nil {
		return nil, err
	}
	names, err := local.AppNames(h.p)
	if err != nil {
		return nil, err
	}
	custom, err := h.customTools()
	if err != nil {
		return nil, err
	}

	var out []core.Object
	for _, entry := range mapping.Children {
		tool := entry.Get("name")
		if tool == "" {
			// An entry with an empty name is how the client records "no
			// override" after the checkbox is cleared; it is not a mapping.
			continue
		}
		out = append(out, h.build(entry, names, custom))
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	appID, err := h.resolveKey(name)
	if err != nil {
		return core.Object{}, err
	}
	mapping, _, err := h.mapping()
	if err != nil {
		return core.Object{}, err
	}
	if mapping == nil {
		return core.Object{}, core.NotFound("compattool", name)
	}
	entry := mapping.Find(appID)
	if entry == nil || entry.Get("name") == "" {
		return core.Object{}, core.NotFound("compattool", name)
	}
	names, err := local.AppNames(h.p)
	if err != nil {
		return core.Object{}, err
	}
	custom, err := h.customTools()
	if err != nil {
		return core.Object{}, err
	}
	return h.build(entry, names, custom), nil
}

func (h *Handler) Apply(ctx context.Context, obj core.Object) (core.Result, error) {
	name := obj.Metadata.Name
	spec, ok := obj.Spec.(*CompatToolSpec)
	if !ok || spec == nil {
		return core.Result{}, fmt.Errorf("compattool %q: missing or invalid spec", name)
	}
	if strings.TrimSpace(spec.Tool) == "" {
		return core.Result{}, fmt.Errorf("compattool %q: spec.tool is empty; delete the object to stop forcing a tool", name)
	}
	appID, err := h.resolveKey(name)
	if err != nil {
		return core.Result{}, err
	}

	path, err := local.Path(h.p, provider.InstallConfigFile)
	if err != nil {
		return core.Result{}, err
	}
	root, err := vdf.ReadFile(path)
	if err != nil {
		return core.Result{}, err
	}
	if root == nil {
		return core.Result{}, fmt.Errorf("no config.vdf at %s: run the Steam client once", path)
	}
	mapping := root.Find(compatToolMapping...)
	if mapping == nil {
		mapping = local.EnsureBlock(root, compatToolMapping...)
	}

	entry := mapping.Find(appID)
	action := core.ActionConfigured
	if entry == nil || entry.Get("name") == "" {
		action = core.ActionCreated
	}

	var diff []string
	current := ""
	if entry != nil {
		current = entry.Get("name")
	}
	if current != spec.Tool {
		diff = append(diff, fmt.Sprintf("tool %q -> %q", current, spec.Tool))
	}
	if entry != nil && spec.Config != "" && entry.Get("config") != spec.Config {
		diff = append(diff, fmt.Sprintf("config %q -> %q", entry.Get("config"), spec.Config))
	}
	if entry != nil && spec.Priority != "" && entry.Get("priority") != spec.Priority {
		diff = append(diff, fmt.Sprintf("priority %q -> %q", entry.Get("priority"), spec.Priority))
	}
	if len(diff) == 0 && action == core.ActionConfigured {
		obj, err := h.Get(ctx, appID)
		return core.Result{Action: core.ActionUnchanged, Object: obj}, err
	}

	if err := local.CheckWritable(h.p, "compatibility tools"); err != nil {
		return core.Result{}, err
	}
	mapping.Set(spec.Tool, appID, "name")
	if spec.Config != "" {
		mapping.Set(spec.Config, appID, "config")
	}
	if spec.Priority != "" {
		mapping.Set(spec.Priority, appID, "priority")
	}
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return core.Result{Action: action, Object: obj, Diff: diff}, nil
	}
	if err := vdf.WriteFile(path, root); err != nil {
		return core.Result{}, err
	}
	updated, err := h.Get(ctx, appID)
	return core.Result{Action: action, Object: updated, Diff: diff}, err
}

// Delete stops forcing a tool, which returns the app to whatever Steam Play
// would have chosen on its own.
func (h *Handler) Delete(ctx context.Context, name string) error {
	appID, err := h.resolveKey(name)
	if err != nil {
		return err
	}
	if _, err := h.Get(ctx, appID); err != nil {
		return err
	}
	if err := local.CheckWritable(h.p, "compatibility tools"); err != nil {
		return err
	}
	path, err := local.Path(h.p, provider.InstallConfigFile)
	if err != nil {
		return err
	}
	root, err := vdf.ReadFile(path)
	if err != nil || root == nil {
		return err
	}
	mapping := root.Find(compatToolMapping...)
	if mapping == nil {
		return core.NotFound("compattool", name)
	}
	mapping.Delete(appID)
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return nil
	}
	return vdf.WriteFile(path, root)
}

func (h *Handler) mapping() (*vdf.Node, string, error) {
	path, err := local.Path(h.p, provider.InstallConfigFile)
	if err != nil {
		return nil, "", err
	}
	root, err := vdf.ReadFile(path)
	if err != nil || root == nil {
		return nil, path, err
	}
	return root.Find(compatToolMapping...), path, nil
}

// resolveKey accepts an app id, an installed game's name, or "default" for the
// machine-wide mapping Steam stores under the key 0.
func (h *Handler) resolveKey(name string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, "default") || name == globalCompatKey {
		return globalCompatKey, nil
	}
	return local.ResolveAppID(h.p, name)
}

// customTools lists what the user has installed under compatibilitytools.d, so
// status can say whether a mapping points at a Valve build or a third-party one.
func (h *Handler) customTools() (map[string]bool, error) {
	root, err := h.p.SteamRoot()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(root, "compatibilitytools.d"))
	if err != nil {
		// The directory only exists once something has been installed into it.
		return out, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			out[e.Name()] = true
		}
	}
	return out, nil
}

func (h *Handler) build(entry *vdf.Node, names map[string]string, custom map[string]bool) core.Object {
	tool := entry.Get("name")
	t := h.Type()
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: entry.Key},
		Spec: &CompatToolSpec{
			Tool:     tool,
			Config:   entry.Get("config"),
			Priority: entry.Get("priority"),
		},
		Status: &CompatToolStatus{
			AppID:    entry.Key,
			AppName:  names[entry.Key],
			Tool:     tool,
			Config:   entry.Get("config"),
			Priority: entry.Get("priority"),
			Global:   entry.Key == globalCompatKey,
			Custom:   custom[tool],
			File:     h.configFile(),
		},
	}
}

func (h *Handler) configFile() string {
	path, err := local.Path(h.p, provider.InstallConfigFile)
	if err != nil {
		return ""
	}
	return path
}

func compatStatus(o core.Object) *CompatToolStatus {
	if s, ok := o.Status.(*CompatToolStatus); ok && s != nil {
		return s
	}
	return &CompatToolStatus{}
}
