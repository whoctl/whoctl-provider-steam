// Package steam assembles the provider from its resources.
//
// The kinds are grouped by which half of Steam they come from: resources/local
// reads and rewrites the client's own KeyValues files, resources/web asks the
// public Web API. A family's shared code sits at its root.
//
// Two web kinds import the local family, and that arrow is the point of showing
// it: OwnedGame's "installed" field is the local half answering, and Achievement
// accepts the name of a game installed on this machine. The halves inform each
// other on purpose.
package steam

import (
	"github.com/whoctl/whoctl-sdk-go/core"

	"github.com/whoctl/whoctl-provider-steam/resources/local/account"
	"github.com/whoctl/whoctl-provider-steam/resources/local/app"
	"github.com/whoctl/whoctl-provider-steam/resources/local/compattool"
	"github.com/whoctl/whoctl-provider-steam/resources/local/launchoption"
	"github.com/whoctl/whoctl-provider-steam/resources/local/libraryfolder"
	"github.com/whoctl/whoctl-provider-steam/resources/local/shortcut"
	"github.com/whoctl/whoctl-provider-steam/resources/web/achievement"
	"github.com/whoctl/whoctl-provider-steam/resources/web/friend"
	"github.com/whoctl/whoctl-provider-steam/resources/web/ownedgame"
	"github.com/whoctl/whoctl-provider-steam/resources/web/profile"
	"github.com/whoctl/whoctl-provider-steam/resources/web/wishlistitem"

	"github.com/whoctl/whoctl-provider-steam/internal/provider"
)

// Options configures the provider.
type Options = provider.Options

// Provider is the steam provider: the shared state, plus the kinds served over
// it.
type Provider struct {
	*provider.Provider
}

// New builds it.
func New(opts Options) *Provider {
	return &Provider{Provider: provider.New(opts)}
}

// Handlers implements core.Provider: the local half first, then the Web API
// half — the order `api-resources` shows them in.
func (p *Provider) Handlers() []core.Handler {
	return []core.Handler{
		app.New(p.Provider),
		libraryfolder.New(p.Provider),
		launchoption.New(p.Provider),
		compattool.New(p.Provider),
		account.New(p.Provider),
		shortcut.New(p.Provider),
		ownedgame.New(p.Provider),
		wishlistitem.New(p.Provider),
		friend.New(p.Provider),
		achievement.New(p.Provider),
		profile.New(p.Provider),
	}
}
