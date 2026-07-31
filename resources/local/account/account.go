// Package account manages the Steam logins recorded on this machine.
package account

import (
	"context"
	"fmt"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/local/vdf"
	"github.com/whoctl/whoctl-sdk-go/core"
)

// AccountSpec is the desired state of a signed-in Steam account.
//
// Note what is not here: no password, no token, nothing that could authenticate
// as the user. loginusers.vdf holds only flags about how the client behaves at
// startup, and the refresh token that lives alongside them in newer clients is
// never read or written by whoctl — the same rule the linux provider applies to
// password hashes, for the same reason.
type AccountSpec struct {
	AutoLogin        *bool `yaml:"autoLogin,omitempty" json:"autoLogin,omitempty" doc:"Whether the client signs this account in automatically at startup. Only one account can have it."`
	RememberPassword *bool `yaml:"rememberPassword,omitempty" json:"rememberPassword,omitempty" doc:"Whether the client keeps this account's credentials for the next launch."`
	OfflineMode      *bool `yaml:"offlineMode,omitempty" json:"offlineMode,omitempty" doc:"Whether the client starts this account in offline mode."`
}

// AccountStatus is the observed state of the account.
type AccountStatus struct {
	AccountName      string `yaml:"accountName" json:"accountName" doc:"The login name, same as metadata.name." docExample:"gaben"`
	PersonaName      string `yaml:"personaName,omitempty" json:"personaName,omitempty" doc:"The display name shown to friends." docExample:"Gabe"`
	SteamID64        string `yaml:"steamId64" json:"steamId64" doc:"The account's 64-bit SteamID." docExample:"76561197960287930"`
	AccountID        string `yaml:"accountId" json:"accountId" doc:"The 32-bit account id, which names the directory under userdata." docExample:"22202"`
	AutoLogin        bool   `yaml:"autoLogin" json:"autoLogin" doc:"Whether this account signs in automatically."`
	RememberPassword bool   `yaml:"rememberPassword" json:"rememberPassword" doc:"Whether credentials are kept between launches."`
	OfflineMode      bool   `yaml:"offlineMode" json:"offlineMode" doc:"Whether the client starts offline for this account."`
	MostRecent       bool   `yaml:"mostRecent" json:"mostRecent" doc:"Whether this is the account the client last used, which is the one the per-user kinds act on by default."`
	LastLogin        int64  `yaml:"lastLogin,omitempty" json:"lastLogin,omitempty" doc:"Unix time of the last sign-in."`
}

// Handler serves the kind.
type Handler struct{ p *provider.Provider }

// New builds the handler.
func New(p *provider.Provider) core.Handler { return &Handler{p: p} }

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "Account",
		Plural:      "accounts",
		Singular:    "account",
		ShortNames:  []string{"acct"},
		Description: "A Steam account signed in on this machine, from config/loginusers.vdf.",
		Columns: []core.Column{
			{Name: "NAME", Path: "metadata.name"},
			{Name: "PERSONA", Path: "status.personaName"},
			{Name: "AUTOLOGIN", Path: "status.autoLogin"},
			{Name: "RECENT", Path: "status.mostRecent"},
			{Name: "STEAMID64", Wide: true, Path: "status.steamId64"},
			{Name: "ACCOUNTID", Wide: true, Path: "status.accountId"},
			{Name: "OFFLINE", Wide: true, Path: "status.offlineMode"},
		},
	})
}

func (h *Handler) NewSpec() any { return &AccountSpec{} }

func (h *Handler) NewStatus() any { return &AccountStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	users, err := local.LoginUsers(h.p)
	if err != nil {
		return nil, err
	}
	recent, _ := local.MostRecentSteamID(h.p)
	out := make([]core.Object, 0, len(users))
	for _, u := range users {
		out = append(out, h.build(u, recent))
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	u, err := h.lookup(name)
	if err != nil {
		return core.Object{}, err
	}
	recent, _ := local.MostRecentSteamID(h.p)
	return h.build(u, recent), nil
}

// Apply reconciles the startup flags. It never creates an account: signing in
// is an authentication whoctl has no part in, and writing a name into
// loginusers.vdf produces an entry the client discards on the next launch.
func (h *Handler) Apply(ctx context.Context, obj core.Object) (core.Result, error) {
	name := obj.Metadata.Name
	spec, ok := obj.Spec.(*AccountSpec)
	if !ok || spec == nil {
		return core.Result{}, fmt.Errorf("account %q: missing or invalid spec", name)
	}
	user, err := h.lookup(name)
	if err != nil {
		if core.IsNotFound(err) {
			return core.Result{}, fmt.Errorf("account %q has not signed in on this machine: sign in with the client first, then whoctl can manage how it starts", name)
		}
		return core.Result{}, err
	}

	path, err := local.Path(h.p, provider.LoginUsersFile)
	if err != nil {
		return core.Result{}, err
	}
	root, err := vdf.ReadFile(path)
	if err != nil || root == nil {
		return core.Result{}, err
	}
	entry := root.Find(user.SteamID64)
	if entry == nil {
		return core.Result{}, core.NotFound("account", name)
	}

	var diff []string
	set := func(key string, want *bool, current bool, label string) {
		if want == nil || *want == current {
			return
		}
		entry.Set(local.BoolToVDF(*want), key)
		diff = append(diff, fmt.Sprintf("%s %t -> %t", label, current, *want))
	}
	set("AutoLogin", spec.AutoLogin, user.AutoLogin, "autoLogin")
	set("RememberPassword", spec.RememberPassword, user.RememberPassword, "rememberPassword")
	set("WantsOfflineMode", spec.OfflineMode, user.WantsOfflineMode, "offlineMode")

	// Steam signs in exactly one account automatically, so turning it on here
	// has to turn it off everywhere else or the client picks arbitrarily.
	if spec.AutoLogin != nil && *spec.AutoLogin {
		for _, other := range root.Children {
			if other.Key != user.SteamID64 && other.GetBool("AutoLogin") {
				other.Set("0", "AutoLogin")
				diff = append(diff, fmt.Sprintf("autoLogin off for %s", other.Get("AccountName")))
			}
		}
	}

	if len(diff) == 0 {
		obj, err := h.Get(ctx, name)
		return core.Result{Action: core.ActionUnchanged, Object: obj}, err
	}
	if err := local.CheckWritable(h.p, fmt.Sprintf("account %q", name)); err != nil {
		return core.Result{}, err
	}
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return core.Result{Action: core.ActionConfigured, Object: obj, Diff: diff}, nil
	}
	if err := vdf.WriteFile(path, root); err != nil {
		return core.Result{}, err
	}
	updated, err := h.Get(ctx, name)
	return core.Result{Action: core.ActionConfigured, Object: updated, Diff: diff}, err
}

// Delete forgets an account: it removes the entry so the client stops offering
// the name at the sign-in prompt. The account itself is untouched, since
// nothing here belongs to Valve's side of the world.
func (h *Handler) Delete(ctx context.Context, name string) error {
	user, err := h.lookup(name)
	if err != nil {
		return err
	}
	if err := local.CheckWritable(h.p, fmt.Sprintf("account %q", name)); err != nil {
		return err
	}
	path, err := local.Path(h.p, provider.LoginUsersFile)
	if err != nil {
		return err
	}
	root, err := vdf.ReadFile(path)
	if err != nil || root == nil {
		return err
	}
	root.Delete(user.SteamID64)
	if !h.p.Runner.Mutate(fmt.Sprintf("rewrite %s", path)) {
		return nil
	}
	return vdf.WriteFile(path, root)
}

// lookup accepts the login name, the persona name or either form of the id,
// because all four are things a user has in front of them.
func (h *Handler) lookup(name string) (local.LoginUser, error) {
	users, err := local.LoginUsers(h.p)
	if err != nil {
		return local.LoginUser{}, err
	}
	accountID, _ := provider.AccountIDFromSteamID64(name)
	for _, u := range users {
		id, _ := provider.AccountIDFromSteamID64(u.SteamID64)
		if u.AccountName == name || u.PersonaName == name || u.SteamID64 == name || (accountID != "" && id == accountID) {
			return u, nil
		}
	}
	return local.LoginUser{}, core.NotFound("account", name)
}

func (h *Handler) build(u local.LoginUser, recent string) core.Object {
	accountID, _ := provider.AccountIDFromSteamID64(u.SteamID64)
	t := h.Type()
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: u.AccountName},
		Spec: &AccountSpec{
			AutoLogin:        provider.BoolPtr(u.AutoLogin),
			RememberPassword: provider.BoolPtr(u.RememberPassword),
			OfflineMode:      provider.BoolPtr(u.WantsOfflineMode),
		},
		Status: &AccountStatus{
			AccountName:      u.AccountName,
			PersonaName:      u.PersonaName,
			SteamID64:        u.SteamID64,
			AccountID:        accountID,
			AutoLogin:        u.AutoLogin,
			RememberPassword: u.RememberPassword,
			OfflineMode:      u.WantsOfflineMode,
			MostRecent:       u.SteamID64 == recent,
			LastLogin:        u.Timestamp,
		},
	}
}

func accountStatus(o core.Object) *AccountStatus {
	if s, ok := o.Status.(*AccountStatus); ok && s != nil {
		return s
	}
	return &AccountStatus{}
}
