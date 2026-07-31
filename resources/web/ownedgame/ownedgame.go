// Package ownedgame is the OwnedGame kind, read from Steam's Web API.
package ownedgame

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/web"
	"github.com/whoctl/whoctl-provider-steam/resources/web/steamapi"
	"github.com/whoctl/whoctl-sdk-go/core"
	"sort"
	"strconv"
	"strings"
)

// --- OwnedGame ---------------------------------------------------------

// OwnedGameSpec is empty: nothing about an owned game is a desired state whoctl
// could reconcile. It exists so the object has the same shape as every other.
type OwnedGameSpec struct{}

// OwnedGameStatus is what the Web API reports about a game in the library.
type OwnedGameStatus struct {
	AppID           string `yaml:"appId" json:"appId" doc:"Steam's numeric id for the game." docExample:"620"`
	Name            string `yaml:"name" json:"name" doc:"The game's title." docExample:"Portal 2"`
	Playtime        int64  `yaml:"playtimeMinutes" json:"playtimeMinutes" doc:"Total playtime in minutes, across every platform."`
	Playtime2Weeks  int64  `yaml:"playtime2WeeksMinutes,omitempty" json:"playtime2WeeksMinutes,omitempty" doc:"Playtime in the last two weeks, in minutes."`
	PlaytimeLinux   int64  `yaml:"playtimeLinuxMinutes,omitempty" json:"playtimeLinuxMinutes,omitempty" doc:"Playtime on Linux, in minutes."`
	PlaytimeWindows int64  `yaml:"playtimeWindowsMinutes,omitempty" json:"playtimeWindowsMinutes,omitempty" doc:"Playtime on Windows, in minutes."`
	PlaytimeDeck    int64  `yaml:"playtimeDeckMinutes,omitempty" json:"playtimeDeckMinutes,omitempty" doc:"Playtime on a Steam Deck, in minutes."`
	LastPlayed      int64  `yaml:"lastPlayed,omitempty" json:"lastPlayed,omitempty" doc:"Unix time the game was last played."`
	Installed       bool   `yaml:"installed" json:"installed" doc:"Whether the game is also installed on this machine, which is the local half of the provider answering."`
	HasStats        bool   `yaml:"hasStats" json:"hasStats" doc:"Whether the game reports achievements and stats."`
}

// Handler serves the kind.
type Handler struct {
	p *provider.Provider
	web.ReadOnly
}

// New builds the handler.
func New(p *provider.Provider) core.Handler {
	return &Handler{p: p, ReadOnly: web.ReadOnly{Kind: "OwnedGame", Reason: web.NoWriteAPI}}
}

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "OwnedGame",
		Plural:      "ownedgames",
		Singular:    "ownedgame",
		ShortNames:  []string{"owned"},
		Description: "A game the account owns, from the Web API. Read-only.",
		Columns: []core.Column{
			{Name: "APPID", Path: "metadata.name"},
			{Name: "NAME", Path: "status.name"},
			{Name: "PLAYTIME", Path: "status.playtimeMinutes", Format: core.FormatMinutes},
			{Name: "INSTALLED", Path: "status.installed"},
			{Name: "2WEEKS", Wide: true, Path: "status.playtime2WeeksMinutes", Format: core.FormatMinutes},
			{Name: "LINUX", Wide: true, Path: "status.playtimeLinuxMinutes", Format: core.FormatMinutes},
		},
	})
}

func (h *Handler) NewSpec() any { return &OwnedGameSpec{} }

func (h *Handler) NewStatus() any { return &OwnedGameStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	client, err := web.API(h.p)
	if err != nil {
		return nil, err
	}
	games, err := client.OwnedGames(ctx)
	if err != nil {
		return nil, err
	}
	installed, err := local.AppManifests(h.p)
	if err != nil {
		return nil, err
	}
	// Most played first: a library of a thousand titles is unreadable in the
	// order Steam happens to return it.
	sort.SliceStable(games, func(i, j int) bool { return games[i].PlaytimeForever > games[j].PlaytimeForever })

	out := make([]core.Object, 0, len(games))
	for _, g := range games {
		out = append(out, h.build(g, installed))
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	objs, err := h.List(ctx)
	if err != nil {
		return core.Object{}, err
	}
	for _, o := range objs {
		if o.Metadata.Name == name || strings.EqualFold(ownedStatus(o).Name, name) {
			return o, nil
		}
	}
	return core.Object{}, core.NotFound("ownedgame", name)
}

func (h *Handler) build(g steamapi.Game, installed map[string]local.ManifestRef) core.Object {
	id := strconv.FormatInt(g.AppID, 10)
	_, isInstalled := installed[id]
	t := h.Type()
	return core.Object{
		APIVersion: t.APIVersion(),
		Kind:       t.Kind,
		Metadata:   core.Metadata{Name: id},
		Spec:       &OwnedGameSpec{},
		Status: &OwnedGameStatus{
			AppID: id, Name: g.Name,
			Playtime: g.PlaytimeForever, Playtime2Weeks: g.Playtime2Weeks,
			PlaytimeLinux: g.PlaytimeLinux, PlaytimeWindows: g.PlaytimeWindows,
			PlaytimeDeck: g.PlaytimeDeck, LastPlayed: g.RTimeLastPlayed,
			Installed: isInstalled, HasStats: g.HasCommunityVisibleStats,
		},
	}
}

func ownedStatus(o core.Object) *OwnedGameStatus {
	if s, ok := o.Status.(*OwnedGameStatus); ok && s != nil {
		return s
	}
	return &OwnedGameStatus{}
}
