// Package profile is the Profile kind, read from Steam's Web API.
package profile

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/web"
	"github.com/whoctl/whoctl-sdk-go/core"
)

// --- Profile -----------------------------------------------------------

// ProfileSpec is empty for the same reason the others are.
type ProfileSpec struct{}

// ProfileStatus is the account's public profile.
type ProfileStatus struct {
	SteamID     string `yaml:"steamId64" json:"steamId64" doc:"The 64-bit SteamID."`
	PersonaName string `yaml:"personaName" json:"personaName" doc:"The display name." docExample:"Gabe"`
	RealName    string `yaml:"realName,omitempty" json:"realName,omitempty" doc:"The real name on the profile, when it is public."`
	State       string `yaml:"state" json:"state" doc:"Online, offline, away and so on."`
	Visibility  string `yaml:"visibility" json:"visibility" doc:"Whether the profile is public or private, which decides what the rest of these kinds can read." docExample:"public"`
	Playing     string `yaml:"playing,omitempty" json:"playing,omitempty" doc:"What the account is playing right now."`
	Country     string `yaml:"country,omitempty" json:"country,omitempty" doc:"The country on the profile."`
	ProfileURL  string `yaml:"profileUrl,omitempty" json:"profileUrl,omitempty" doc:"Link to the community profile."`
	Level       int    `yaml:"level,omitempty" json:"level,omitempty" doc:"The Steam community level."`
	Created     int64  `yaml:"created,omitempty" json:"created,omitempty" doc:"Unix time the account was created."`
	LastLogoff  int64  `yaml:"lastLogoff,omitempty" json:"lastLogoff,omitempty" doc:"Unix time of the last sign-out."`
}

// Handler serves the kind.
type Handler struct {
	p *provider.Provider
	web.ReadOnly
}

// New builds the handler.
func New(p *provider.Provider) core.Handler {
	return &Handler{p: p, ReadOnly: web.ReadOnly{Kind: "Profile", Reason: web.NoWriteAPI}}
}

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "Profile",
		Plural:      "profiles",
		Singular:    "profile",
		ShortNames:  []string{"me"},
		Description: "The account's public Steam profile, from the Web API. Read-only.",
		Columns: []core.Column{
			{Name: "STEAMID", Path: "metadata.name"},
			{Name: "PERSONA", Path: "status.personaName"},
			{Name: "STATE", Path: "status.state"},
			{Name: "VISIBILITY", Path: "status.visibility"},
			{Name: "LEVEL", Path: "status.level"},
			{Name: "PLAYING", Wide: true, Path: "status.playing"},
			{Name: "URL", Wide: true, Path: "status.profileUrl"},
		},
	})
}

func (h *Handler) NewSpec() any { return &ProfileSpec{} }

func (h *Handler) NewStatus() any { return &ProfileStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	obj, err := h.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	return []core.Object{obj}, nil
}

// Get looks up the configured account, or any SteamID given by name — a profile
// is public information, so reading someone else's is the same call.
func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	client, err := web.API(h.p)
	if err != nil {
		return core.Object{}, err
	}
	id := name
	if id == "" {
		id = client.SteamID()
	}
	players, err := client.PlayerSummaries(ctx, []string{id})
	if err != nil {
		return core.Object{}, err
	}
	if len(players) == 0 {
		return core.Object{}, core.NotFound("profile", id)
	}
	p := players[0]

	level := 0
	if id == client.SteamID() {
		// The level call only answers for the configured account.
		level, _ = client.SteamLevel(ctx)
	}
	t := h.Type()
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: p.SteamID},
		Spec:       &ProfileSpec{},
		Status: &ProfileStatus{
			SteamID: p.SteamID, PersonaName: p.PersonaName, RealName: p.RealName,
			State: web.PersonaState(p.PersonaState), Visibility: visibility(p.CommunityVisibilityState),
			Playing: p.GameExtraInfo, Country: p.LocCountryCode, ProfileURL: p.ProfileURL,
			Level: level, Created: p.TimeCreated, LastLogoff: p.LastLogoff,
		},
	}, nil
}

// visibility turns Steam's number into the word that decides whether the other
// web kinds can read anything at all.
func visibility(v int) string {
	if v == 3 {
		return "public"
	}
	return "private"
}
