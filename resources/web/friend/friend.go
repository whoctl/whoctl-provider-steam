// Package friend is the Friend kind, read from Steam's Web API.
package friend

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/web"
	"github.com/whoctl/whoctl-provider-steam/resources/web/steamapi"
	"github.com/whoctl/whoctl-sdk-go/core"
	"sort"
	"strings"
)

// --- Friend ------------------------------------------------------------

// FriendSpec is empty for the same reason the others are.
type FriendSpec struct{}

// FriendStatus is what the Web API reports about a friend.
type FriendStatus struct {
	SteamID     string `yaml:"steamId64" json:"steamId64" doc:"The friend's 64-bit SteamID, same as metadata.name."`
	PersonaName string `yaml:"personaName,omitempty" json:"personaName,omitempty" doc:"The friend's display name." docExample:"Gabe"`
	State       string `yaml:"state" json:"state" doc:"Online, offline, away, busy and so on." docExample:"online"`
	Playing     string `yaml:"playing,omitempty" json:"playing,omitempty" doc:"What the friend is playing right now, when their profile shows it."`
	FriendSince int64  `yaml:"friendSince,omitempty" json:"friendSince,omitempty" doc:"Unix time the friendship started."`
	ProfileURL  string `yaml:"profileUrl,omitempty" json:"profileUrl,omitempty" doc:"Link to the friend's community profile."`
	Country     string `yaml:"country,omitempty" json:"country,omitempty" doc:"The country on the friend's profile, when it is public."`
}

// Handler serves the kind.
type Handler struct {
	p *provider.Provider
	web.ReadOnly
}

// New builds the handler.
func New(p *provider.Provider) core.Handler {
	return &Handler{p: p, ReadOnly: web.ReadOnly{Kind: "Friend", Reason: web.NoWriteAPI}}
}

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "Friend",
		Plural:      "friends",
		Singular:    "friend",
		ShortNames:  []string{"fr"},
		Description: "Someone on the account's friend list, from the Web API. Read-only.",
		Columns: []core.Column{
			{Name: "STEAMID", Path: "metadata.name"},
			{Name: "PERSONA", Path: "status.personaName"},
			{Name: "STATE", Path: "status.state"},
			{Name: "PLAYING", Path: "status.playing"},
			{Name: "SINCE", Wide: true, Path: "status.friendSince"},
			{Name: "COUNTRY", Wide: true, Path: "status.country"},
		},
	})
}

func (h *Handler) NewSpec() any { return &FriendSpec{} }

func (h *Handler) NewStatus() any { return &FriendStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	client, err := web.API(h.p)
	if err != nil {
		return nil, err
	}
	friends, err := client.Friends(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(friends))
	for _, f := range friends {
		ids = append(ids, f.SteamID)
	}
	// The friend list carries ids only, so the names come from a second call.
	summaries := map[string]steamapi.Player{}
	if players, err := client.PlayerSummaries(ctx, ids); err == nil {
		for _, p := range players {
			summaries[p.SteamID] = p
		}
	}

	t := h.Type()
	out := make([]core.Object, 0, len(friends))
	for _, f := range friends {
		p := summaries[f.SteamID]
		out = append(out, core.Object{
			APIVersion: t.APIVersion(),
			Kind:       t.Kind,
			Metadata:   core.Metadata{Name: f.SteamID},
			Spec:       &FriendSpec{},
			Status: &FriendStatus{
				SteamID: f.SteamID, PersonaName: p.PersonaName, State: web.PersonaState(p.PersonaState),
				Playing: p.GameExtraInfo, FriendSince: f.FriendSince,
				ProfileURL: p.ProfileURL, Country: p.LocCountryCode,
			},
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return friendStatus(out[i]).PersonaName < friendStatus(out[j]).PersonaName
	})
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	objs, err := h.List(ctx)
	if err != nil {
		return core.Object{}, err
	}
	for _, o := range objs {
		if o.Metadata.Name == name || strings.EqualFold(friendStatus(o).PersonaName, name) {
			return o, nil
		}
	}
	return core.Object{}, core.NotFound("friend", name)
}

func friendStatus(o core.Object) *FriendStatus {
	if s, ok := o.Status.(*FriendStatus); ok && s != nil {
		return s
	}
	return &FriendStatus{}
}
