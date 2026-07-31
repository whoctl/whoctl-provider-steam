// Package launchoption manages the per-game launch command.
package launchoption

import (
	"context"
	"fmt"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
	"strings"
)

// localConfigPath is where the client stores per-account, per-app settings.
var localConfigApps = []string{"Software", "Valve", "Steam", "apps"}

const localConfigFile = "config/localconfig.vdf"

// LaunchOptionSpec is the desired launch command for one app.
//
// This is the field behind "Launch Options" in the client's properties dialog:
// a command line where %command% stands for the game's own executable, which is
// how wrappers like mangohud, gamemoderun and Proton overrides are applied.
type LaunchOptionSpec struct {
	Options string `yaml:"options" json:"options" doc:"The launch command. %command% is replaced by the game's own executable, so a wrapper goes before it and extra arguments after." docFlags:"required" docExample:"mangohud %command%"`
	Account string `yaml:"account,omitempty" json:"account,omitempty" doc:"Which signed-in account to set this for. Defaults to the one the client used last." docExample:"gaben"`
}

// LaunchOptionStatus is the observed state.
type LaunchOptionStatus struct {
	AppID   string `yaml:"appId" json:"appId" doc:"The app the options belong to, same as metadata.name." docExample:"2200"`
	AppName string `yaml:"appName,omitempty" json:"appName,omitempty" doc:"The app's display name, when it is installed on this machine." docExample:"Quake III Arena"`
	Options string `yaml:"options" json:"options" doc:"The launch command currently set." docExample:"mangohud %command%"`
	Account string `yaml:"account" json:"account" doc:"The account whose configuration this came from." docExample:"gaben"`
	File    string `yaml:"file" json:"file" doc:"The localconfig.vdf the value was read from."`
}

// Handler serves the kind.
type Handler struct{ p *provider.Provider }

// New builds the handler.
func New(p *provider.Provider) core.Handler { return &Handler{p: p} }

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "LaunchOption",
		Plural:      "launchoptions",
		Singular:    "launchoption",
		ShortNames:  []string{"launchopt"},
		Description: "The launch command for one app, from the account's localconfig.vdf.",
		Columns: []core.Column{
			{Name: "APPID", Path: "metadata.name"},
			{Name: "APP", Path: "status.appName"},
			{Name: "OPTIONS", Path: "status.options"},
			{Name: "ACCOUNT", Wide: true, Path: "status.account"},
		},
	})
}

func (h *Handler) NewSpec() any { return &LaunchOptionSpec{} }

func (h *Handler) NewStatus() any { return &LaunchOptionStatus{} }

// List reports only the apps that actually have options set. localconfig.vdf
// has an entry for every app the account has ever touched, and listing hundreds
// of empty strings would bury the handful that mean something.
func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	apps, account, path, err := h.appsNode("")
	if err != nil || apps == nil {
		return nil, err
	}
	names, err := local.AppNames(h.p)
	if err != nil {
		return nil, err
	}

	var out []core.Object
	for _, app := range apps.Children {
		options := app.Get("LaunchOptions")
		if strings.TrimSpace(options) == "" {
			continue
		}
		out = append(out, h.build(app.Key, names[app.Key], options, account, path))
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	appID, err := local.ResolveAppID(h.p, name)
	if err != nil {
		return core.Object{}, err
	}
	apps, account, path, err := h.appsNode("")
	if err != nil {
		return core.Object{}, err
	}
	options := ""
	if apps != nil {
		options = apps.Get(appID, "LaunchOptions")
	}
	if strings.TrimSpace(options) == "" {
		return core.Object{}, core.NotFound("launchoption", name)
	}
	names, err := local.AppNames(h.p)
	if err != nil {
		return core.Object{}, err
	}
	return h.build(appID, names[appID], options, account, path), nil
}

func (h *Handler) Apply(ctx context.Context, obj core.Object) (core.Result, error) {
	name := obj.Metadata.Name
	spec, ok := obj.Spec.(*LaunchOptionSpec)
	if !ok || spec == nil {
		return core.Result{}, fmt.Errorf("launchoption %q: missing or invalid spec", name)
	}
	if strings.TrimSpace(spec.Options) == "" {
		return core.Result{}, fmt.Errorf("launchoption %q: spec.options is empty; delete the object to clear it", name)
	}
	appID, err := local.ResolveAppID(h.p, name)
	if err != nil {
		return core.Result{}, err
	}

	path, root, apps, account, err := h.openConfig(spec.Account)
	if err != nil {
		return core.Result{}, err
	}
	current := apps.Get(appID, "LaunchOptions")
	if current == spec.Options {
		obj, err := h.Get(ctx, appID)
		return core.Result{Action: core.ActionUnchanged, Object: obj}, err
	}

	action := core.ActionConfigured
	if strings.TrimSpace(current) == "" {
		action = core.ActionCreated
	}
	if err := local.CheckWritable(h.p, "launch options"); err != nil {
		return core.Result{}, err
	}
	// Set creates the app's block when the account has never launched the game,
	// which is exactly when someone wants to set options in advance.
	apps.Set(spec.Options, appID, "LaunchOptions")
	diff := []string{fmt.Sprintf("options %q -> %q", current, spec.Options)}

	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return core.Result{Action: action, Object: obj, Diff: diff}, nil
	}
	if err := vdf.WriteFile(path, root); err != nil {
		return core.Result{}, err
	}
	_ = account
	updated, err := h.Get(ctx, appID)
	return core.Result{Action: action, Object: updated, Diff: diff}, err
}

// Delete clears the launch command. The app's own block stays, because Steam
// keeps playtime and other bookkeeping in it that has nothing to do with whoctl.
func (h *Handler) Delete(ctx context.Context, name string) error {
	appID, err := local.ResolveAppID(h.p, name)
	if err != nil {
		return err
	}
	path, root, apps, _, err := h.openConfig("")
	if err != nil {
		return err
	}
	if strings.TrimSpace(apps.Get(appID, "LaunchOptions")) == "" {
		return core.NotFound("launchoption", name)
	}
	if err := local.CheckWritable(h.p, "launch options"); err != nil {
		return err
	}
	apps.Delete(appID, "LaunchOptions")
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return nil
	}
	return vdf.WriteFile(path, root)
}

// appsNode returns the apps block for an account, read-only.
func (h *Handler) appsNode(account string) (*vdf.Node, string, string, error) {
	path, name, err := local.ConfigPath(h.p, account)
	if err != nil {
		return nil, "", "", err
	}
	root, err := vdf.ReadFile(path)
	if err != nil || root == nil {
		return nil, name, path, err
	}
	return root.Find(localConfigApps...), name, path, nil
}

// openConfig returns everything a write needs, creating the apps block when the
// file does not have one yet.
func (h *Handler) openConfig(account string) (string, *vdf.Node, *vdf.Node, string, error) {
	path, name, err := local.ConfigPath(h.p, account)
	if err != nil {
		return "", nil, nil, "", err
	}
	root, err := vdf.ReadFile(path)
	if err != nil {
		return "", nil, nil, "", err
	}
	if root == nil {
		return "", nil, nil, "", fmt.Errorf("no localconfig.vdf for account %q: sign in with the client once so Steam creates it", name)
	}
	apps := root.Find(localConfigApps...)
	if apps == nil {
		apps = local.EnsureBlock(root, localConfigApps...)
	}
	return path, root, apps, name, nil
}

func (h *Handler) resolveAppID(name string) (string, error) {
	return local.ResolveAppID(h.p, name)
}

func (h *Handler) build(appID, appName, options, account, path string) core.Object {
	t := h.Type()
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: appID},
		Spec:       &LaunchOptionSpec{Options: options},
		Status: &LaunchOptionStatus{
			AppID: appID, AppName: appName, Options: options, Account: account, File: path,
		},
	}
}
