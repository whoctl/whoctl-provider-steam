// Package achievement is the Achievement kind, read from Steam's Web API.
package achievement

import (
	"context"
	"fmt"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/web"
	"github.com/whoctl/whoctl-sdk-go/core"
)

// --- Achievement -------------------------------------------------------

// AchievementSpec is empty for the same reason the others are.
type AchievementSpec struct{}

// AchievementStatus is what the Web API reports about one achievement.
type AchievementStatus struct {
	APIName     string `yaml:"apiName" json:"apiName" doc:"The achievement's internal name, same as metadata.name." docExample:"ACH_WIN_ONE_GAME"`
	DisplayName string `yaml:"displayName,omitempty" json:"displayName,omitempty" doc:"The name shown in the client."`
	Description string `yaml:"description,omitempty" json:"description,omitempty" doc:"What the achievement asks for. Hidden achievements have none until unlocked."`
	Unlocked    bool   `yaml:"unlocked" json:"unlocked" doc:"Whether the account has it."`
	UnlockTime  int64  `yaml:"unlockTime,omitempty" json:"unlockTime,omitempty" doc:"Unix time it was unlocked."`
	AppID       string `yaml:"appId" json:"appId" doc:"The game the achievement belongs to."`
}

// Handler serves the kind.
type Handler struct {
	p *provider.Provider
	web.ReadOnly
}

// New builds the handler.
func New(p *provider.Provider) core.Handler {
	return &Handler{p: p, ReadOnly: web.ReadOnly{Kind: "Achievement", Reason: web.NoWriteAPI}}
}

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "Achievement",
		Plural:      "achievements",
		Singular:    "achievement",
		ShortNames:  []string{"ach"},
		Description: "An achievement for one game, from the Web API. Needs an app id and is read-only.",
		Columns: []core.Column{
			{Name: "NAME", Path: "metadata.name"},
			{Name: "DISPLAY", Path: "status.displayName"},
			{Name: "UNLOCKED", Path: "status.unlocked"},
			{Name: "APPID", Wide: true, Path: "status.appId"},
			{Name: "DESCRIPTION", Wide: true, Path: "status.description"},
		},
	})
}

func (h *Handler) NewSpec() any { return &AchievementSpec{} }

func (h *Handler) NewStatus() any { return &AchievementStatus{} }

// List is not meaningful without a game: an account owning a thousand titles
// would mean a thousand API calls, and Valve rate limits for a reason. The
// error names the shape of the command that does work.
func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	return nil, fmt.Errorf("achievements are per game: name one, as in `whoctl get steam/achievements 620` or `whoctl get steam/ach \"Portal 2\"`")
}

// Get would have to answer for a bare achievement name, which does not identify
// anything: the api names are unique within a game and nowhere else. The game
// is the addressable unit here, so ListScoped is what serves this kind.
func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	return core.Object{}, fmt.Errorf("%q is an achievement, and an achievement is named only within its game: ask for the game instead, as in `whoctl get steam/achievements 620`", name)
}

// ListScoped implements core.ScopedLister: the argument is a game, and the
// listing is that game's achievements.
func (h *Handler) ListScoped(ctx context.Context, app string) ([]core.Object, error) {
	appID, err := local.ResolveAppID(h.p, app)
	if err != nil {
		return nil, err
	}
	return h.forApp(ctx, appID)
}

// forApp is the real entry point, used by the app-scoped listing.
func (h *Handler) forApp(ctx context.Context, appID string) ([]core.Object, error) {
	client, err := web.API(h.p)
	if err != nil {
		return nil, err
	}
	list, err := client.Achievements(ctx, appID)
	if err != nil {
		return nil, err
	}
	t := h.Type()
	out := make([]core.Object, 0, len(list))
	for _, a := range list {
		out = append(out, core.Object{
			APIVersion: t.APIVersion(),
			Kind:       t.Kind,
			Metadata:   core.Metadata{Name: a.APIName},
			Spec:       &AchievementSpec{},
			Status: &AchievementStatus{
				APIName: a.APIName, DisplayName: a.Name, Description: a.Description,
				Unlocked: a.Achieved != 0, UnlockTime: a.UnlockTime, AppID: appID,
			},
		})
	}
	return out, nil
}
