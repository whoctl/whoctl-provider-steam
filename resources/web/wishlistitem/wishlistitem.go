// Package wishlistitem is the WishlistItem kind, read from Steam's Web API.
package wishlistitem

import (
	"context"
	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/web"
	"github.com/whoctl/whoctl-sdk-go/core"
	"sort"
	"strconv"
)

// --- WishlistItem ------------------------------------------------------

// WishlistItemSpec is empty for the same reason OwnedGameSpec is.
type WishlistItemSpec struct{}

// WishlistItemStatus is what the Web API reports about a wishlisted game.
type WishlistItemStatus struct {
	AppID     string `yaml:"appId" json:"appId" doc:"Steam's numeric id for the game." docExample:"620"`
	Priority  int    `yaml:"priority" json:"priority" doc:"The account's own ordering of the wishlist, 1 being the top."`
	DateAdded int64  `yaml:"dateAdded,omitempty" json:"dateAdded,omitempty" doc:"Unix time the game was added to the wishlist."`
	Owned     bool   `yaml:"owned" json:"owned" doc:"Whether the account already owns the game, which happens when a wishlisted item is bought but not removed."`
}

// Handler serves the kind.
type Handler struct {
	p *provider.Provider
	web.ReadOnly
}

// New builds the handler.
func New(p *provider.Provider) core.Handler {
	return &Handler{p: p, ReadOnly: web.ReadOnly{Kind: "WishlistItem", Reason: web.NoWriteAPI}}
}

func (h *Handler) Type() core.ResourceType {
	return provider.ResourceType(core.ResourceType{
		Kind:        "WishlistItem",
		Plural:      "wishlist",
		Singular:    "wishlistitem",
		ShortNames:  []string{"wish"},
		Description: "A game on the account's wishlist, from the Web API. Read-only.",
		Columns: []core.Column{
			{Name: "APPID", Path: "metadata.name"},
			{Name: "PRIORITY", Path: "status.priority"},
			{Name: "OWNED", Path: "status.owned"},
			{Name: "ADDED", Wide: true, Path: "status.dateAdded"},
		},
	})
}

func (h *Handler) NewSpec() any { return &WishlistItemSpec{} }

func (h *Handler) NewStatus() any { return &WishlistItemStatus{} }

func (h *Handler) List(ctx context.Context) ([]core.Object, error) {
	client, err := web.API(h.p)
	if err != nil {
		return nil, err
	}
	items, err := client.Wishlist(ctx)
	if err != nil {
		return nil, err
	}
	owned := map[string]bool{}
	if games, err := client.OwnedGames(ctx); err == nil {
		for _, g := range games {
			owned[strconv.FormatInt(g.AppID, 10)] = true
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Priority < items[j].Priority })

	t := h.Type()
	out := make([]core.Object, 0, len(items))
	for _, item := range items {
		id := strconv.FormatInt(item.AppID, 10)
		out = append(out, core.Object{
			APIVersion: t.APIVersion(),
			Kind:       t.Kind,
			Metadata:   core.Metadata{Name: id},
			Spec:       &WishlistItemSpec{},
			Status: &WishlistItemStatus{
				AppID: id, Priority: item.Priority, DateAdded: item.DateAdded, Owned: owned[id],
			},
		})
	}
	return out, nil
}

func (h *Handler) Get(ctx context.Context, name string) (core.Object, error) {
	objs, err := h.List(ctx)
	if err != nil {
		return core.Object{}, err
	}
	for _, o := range objs {
		if o.Metadata.Name == name {
			return o, nil
		}
	}
	return core.Object{}, core.NotFound("wishlistitem", name)
}
