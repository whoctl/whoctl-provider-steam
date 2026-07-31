// Package shortcut manages non-Steam games added to the library.
package shortcut

import (
	"context"
	"fmt"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
	"hash/crc32"
	"strconv"
	"strings"
)

const shortcutsFile = "config/shortcuts.vdf"

// ShortcutSpec is a non-Steam game added to the library.
//
// This is the one Steam resource with a full lifecycle: shortcuts are entirely
// the user's, Steam only stores them, and nothing on Valve's side has to agree.
// So unlike App, this kind really does create and delete.
type ShortcutSpec struct {
	Exe                string   `yaml:"exe" json:"exe" doc:"The program to run. Quote it if the path contains spaces, the way Steam itself writes it." docFlags:"required" docExample:"/usr/bin/lutris"`
	StartDir           string   `yaml:"startDir,omitempty" json:"startDir,omitempty" doc:"Working directory for the program. Defaults to the executable's own directory." docExample:"/usr/bin/"`
	LaunchOptions      string   `yaml:"launchOptions,omitempty" json:"launchOptions,omitempty" doc:"Arguments passed to the program." docExample:"rungame/rom"`
	Icon               string   `yaml:"icon,omitempty" json:"icon,omitempty" doc:"Path to an icon file."`
	Tags               []string `yaml:"tags,omitempty" json:"tags,omitempty" doc:"Steam categories the shortcut appears under." docExample:"Emulators"`
	Hidden             *bool    `yaml:"hidden,omitempty" json:"hidden,omitempty" doc:"Whether the shortcut is hidden from the library."`
	AllowOverlay       *bool    `yaml:"allowOverlay,omitempty" json:"allowOverlay,omitempty" doc:"Whether the Steam overlay is enabled for it."`
	AllowDesktopConfig *bool    `yaml:"allowDesktopConfig,omitempty" json:"allowDesktopConfig,omitempty" doc:"Whether the desktop controller configuration applies."`
	Account            string   `yaml:"account,omitempty" json:"account,omitempty" doc:"Which signed-in account to add the shortcut for. Defaults to the one the client used last." docExample:"gaben"`
}

// ShortcutStatus is the observed state.
type ShortcutStatus struct {
	AppName            string   `yaml:"appName" json:"appName" doc:"The shortcut's name, same as metadata.name." docExample:"Lutris"`
	AppID              string   `yaml:"appId" json:"appId" doc:"The id Steam assigns the shortcut, which is derived from the executable and the name."`
	Exe                string   `yaml:"exe" json:"exe" doc:"The program that runs."`
	StartDir           string   `yaml:"startDir,omitempty" json:"startDir,omitempty" doc:"Working directory for the program."`
	LaunchOptions      string   `yaml:"launchOptions,omitempty" json:"launchOptions,omitempty" doc:"Arguments passed to the program."`
	Icon               string   `yaml:"icon,omitempty" json:"icon,omitempty" doc:"Path to the icon file."`
	Tags               []string `yaml:"tags,omitempty" json:"tags,omitempty" doc:"Steam categories the shortcut appears under."`
	Hidden             bool     `yaml:"hidden" json:"hidden" doc:"Whether the shortcut is hidden from the library."`
	AllowOverlay       bool     `yaml:"allowOverlay" json:"allowOverlay" doc:"Whether the Steam overlay is enabled."`
	AllowDesktopConfig bool     `yaml:"allowDesktopConfig" json:"allowDesktopConfig" doc:"Whether the desktop controller configuration applies."`
	LastPlayed         int64    `yaml:"lastPlayed,omitempty" json:"lastPlayed,omitempty" doc:"Unix time the shortcut was last launched."`
	Account            string   `yaml:"account" json:"account" doc:"The account the shortcut belongs to."`
	File               string   `yaml:"file" json:"file" doc:"The shortcuts.vdf the entry was read from."`
}

// Handler serves the kind.
type Handler struct{ p *provider.Provider }

// New builds the handler.
func New(p *provider.Provider) core.Handler { return &Handler{p: p} }

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "Shortcut",
		Plural:      "shortcuts",
		Singular:    "shortcut",
		ShortNames:  []string{"nonsteam"},
		Description: "A non-Steam game added to the library, from the account's shortcuts.vdf.",
		Columns: []core.Column{
			{Name: "NAME", Path: "metadata.name"},
			{Name: "EXE", Path: "status.exe"},
			{Name: "TAGS", Path: "status.tags"},
			{Name: "HIDDEN", Path: "status.hidden"},
			{Name: "APPID", Wide: true, Path: "status.appId"},
			{Name: "STARTDIR", Wide: true, Path: "status.startDir"},
			{Name: "ACCOUNT", Wide: true, Path: "status.account"},
		},
	})
}

func (h *Handler) NewSpec() any { return &ShortcutSpec{} }

func (h *Handler) NewStatus() any { return &ShortcutStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	root, account, path, err := h.read("")
	if err != nil || root == nil {
		return nil, err
	}
	out := make([]core.Object, 0, len(root.Children))
	for _, entry := range root.Children {
		out = append(out, h.build(entry, account, path))
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	root, account, path, err := h.read("")
	if err != nil {
		return core.Object{}, err
	}
	if root != nil {
		for _, entry := range root.Children {
			if strings.EqualFold(entry.Get("AppName"), name) {
				return h.build(entry, account, path), nil
			}
		}
	}
	return core.Object{}, core.NotFound("shortcut", name)
}

func (h *Handler) Apply(ctx context.Context, obj core.Object) (core.Result, error) {
	name := obj.Metadata.Name
	spec, ok := obj.Spec.(*ShortcutSpec)
	if !ok || spec == nil {
		return core.Result{}, fmt.Errorf("shortcut %q: missing or invalid spec", name)
	}
	if strings.TrimSpace(spec.Exe) == "" {
		return core.Result{}, fmt.Errorf("shortcut %q: spec.exe is required", name)
	}

	root, account, path, err := h.read(spec.Account)
	if err != nil {
		return core.Result{}, err
	}
	if root == nil {
		root = &vdf.Node{Key: "shortcuts", Block: true}
	}

	var (
		entry  *vdf.Node
		action = core.ActionCreated
	)
	for _, child := range root.Children {
		if strings.EqualFold(child.Get("AppName"), name) {
			entry, action = child, core.ActionConfigured
			break
		}
	}
	if entry == nil {
		entry = &vdf.Node{Key: strconv.Itoa(len(root.Children)), Block: true}
		entry.Set(shortcutAppID(spec.Exe, name), "appid")
		entry.Set(name, "AppName")
		root.Children = append(root.Children, entry)
	}

	diff := h.reconcile(entry, spec)
	if action == core.ActionConfigured && len(diff) == 0 {
		obj, err := h.Get(ctx, name)
		return core.Result{Action: core.ActionUnchanged, Object: obj}, err
	}
	if err := local.CheckWritable(h.p, "shortcuts"); err != nil {
		return core.Result{}, err
	}
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return core.Result{Action: action, Object: obj, Diff: diff}, nil
	}
	if err := vdf.WriteBinaryFile(path, root); err != nil {
		return core.Result{}, err
	}
	_ = account
	updated, err := h.Get(ctx, name)
	return core.Result{Action: action, Object: updated, Diff: diff}, err
}

// reconcile writes the spec into an entry and reports what changed. Fields the
// manifest omits are left as they are, so a shortcut Steam created keeps the
// flags the client set on it.
func (h *Handler) reconcile(entry *vdf.Node, spec *ShortcutSpec) []string {
	var diff []string
	setString := func(key, want, label string) {
		if want == "" || entry.Get(key) == want {
			return
		}
		diff = append(diff, fmt.Sprintf("%s %q -> %q", label, entry.Get(key), want))
		entry.Set(want, key)
	}
	setFlag := func(key string, want *bool, label string) {
		if want == nil {
			return
		}
		value := local.BoolToVDF(*want)
		if entry.Get(key) == value {
			return
		}
		diff = append(diff, fmt.Sprintf("%s -> %t", label, *want))
		entry.Set(value, key)
	}

	setString("Exe", spec.Exe, "exe")
	setString("StartDir", spec.StartDir, "startDir")
	setString("LaunchOptions", spec.LaunchOptions, "launchOptions")
	setString("icon", spec.Icon, "icon")
	setFlag("IsHidden", spec.Hidden, "hidden")
	setFlag("AllowOverlay", spec.AllowOverlay, "allowOverlay")
	setFlag("AllowDesktopConfig", spec.AllowDesktopConfig, "allowDesktopConfig")

	if spec.Tags != nil && !equalStrings(readTags(entry), spec.Tags) {
		diff = append(diff, fmt.Sprintf("tags %v -> %v", readTags(entry), spec.Tags))
		writeTags(entry, spec.Tags)
	}
	return diff
}

func (h *Handler) Delete(ctx context.Context, name string) error {
	root, _, path, err := h.read("")
	if err != nil {
		return err
	}
	if root == nil {
		return core.NotFound("shortcut", name)
	}
	kept := make([]*vdf.Node, 0, len(root.Children))
	found := false
	for _, entry := range root.Children {
		if strings.EqualFold(entry.Get("AppName"), name) {
			found = true
			continue
		}
		kept = append(kept, entry)
	}
	if !found {
		return core.NotFound("shortcut", name)
	}
	if err := local.CheckWritable(h.p, "shortcuts"); err != nil {
		return err
	}
	// Steam keys shortcuts by consecutive index, so the survivors are
	// renumbered rather than left with a hole the client would misread.
	for i, entry := range kept {
		entry.Key = strconv.Itoa(i)
	}
	root.Children = kept
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return nil
	}
	return vdf.WriteBinaryFile(path, root)
}

func (h *Handler) read(account string) (*vdf.Node, string, string, error) {
	steamID, name, err := local.ResolveAccount(h.p, account)
	if err != nil {
		return nil, "", "", err
	}
	path, err := local.UserDataPath(h.p, steamID, shortcutsFile)
	if err != nil {
		return nil, "", "", err
	}
	root, err := vdf.ReadBinaryFile(path)
	return root, name, path, err
}

func (h *Handler) build(entry *vdf.Node, account, path string) core.Object {
	played, _ := entry.GetInt("LastPlayTime")
	t := h.Type()
	tags := readTags(entry)
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: entry.Get("AppName")},
		Spec: &ShortcutSpec{
			Exe:                entry.Get("Exe"),
			StartDir:           entry.Get("StartDir"),
			LaunchOptions:      entry.Get("LaunchOptions"),
			Icon:               entry.Get("icon"),
			Tags:               tags,
			Hidden:             provider.BoolPtr(entry.GetBool("IsHidden")),
			AllowOverlay:       provider.BoolPtr(entry.GetBool("AllowOverlay")),
			AllowDesktopConfig: provider.BoolPtr(entry.GetBool("AllowDesktopConfig")),
		},
		Status: &ShortcutStatus{
			AppName:            entry.Get("AppName"),
			AppID:              entry.Get("appid"),
			Exe:                entry.Get("Exe"),
			StartDir:           entry.Get("StartDir"),
			LaunchOptions:      entry.Get("LaunchOptions"),
			Icon:               entry.Get("icon"),
			Tags:               tags,
			Hidden:             entry.GetBool("IsHidden"),
			AllowOverlay:       entry.GetBool("AllowOverlay"),
			AllowDesktopConfig: entry.GetBool("AllowDesktopConfig"),
			LastPlayed:         played,
			Account:            account,
			File:               path,
		},
	}
}

// Tags are a nested block keyed by position, which is how Steam stores every
// list in this format.
func readTags(entry *vdf.Node) []string {
	node := entry.Find("tags")
	if node == nil {
		return nil
	}
	out := make([]string, 0, len(node.Children))
	for _, tag := range node.Children {
		out = append(out, tag.Value)
	}
	return out
}

func writeTags(entry *vdf.Node, tags []string) {
	entry.Delete("tags")
	node := &vdf.Node{Key: "tags", Block: true}
	for i, tag := range tags {
		node.Children = append(node.Children, &vdf.Node{Key: strconv.Itoa(i), Value: tag})
	}
	entry.Children = append(entry.Children, node)
}

// shortcutAppID reproduces the id Steam derives for a non-Steam game: a CRC32
// of the executable and name, with the top bit set so it cannot collide with a
// real app id. Getting this right is what lets artwork and controller
// configuration find the shortcut.
func shortcutAppID(exe, name string) string {
	sum := crc32.ChecksumIEEE([]byte(exe + name))
	return strconv.FormatInt(int64(int32(sum|0x80000000)), 10)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
