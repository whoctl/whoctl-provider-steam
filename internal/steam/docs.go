package steam

import (
	_ "embed"

	"github.com/whoctl/whoctl-sdk-go/core"
	"github.com/whoctl/whoctl-sdk-go/docs"

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
)

// The overview is the only page belonging to the provider as a whole; every
// other page lives with the kind it documents.
//
//go:embed index.md
var indexPage string

// Docs implements core.DocumentedProvider.
func (p *Provider) Docs() core.ProviderDocs {
	return core.ProviderDocs{
		DisplayName: "Steam",
		Summary:     "A local Steam installation and the account behind it: installed games, launch options, compatibility tools, and what the Web API says about the player.",
		Categories:  []string{"Desktop"},
		Maturity:    "alpha",
		FS:          docs.Tree(pages()),
		// The kinds are grouped by half, so where a page lives cannot be
		// derived from the kind's singular.
		PagePath:  pagePath,
		SourceDir: "resources",
	}
}

var pageLayout = map[string]string{
	"app":           "local/app/app.md",
	"libraryfolder": "local/libraryfolder/libraryfolder.md",
	"launchoption":  "local/launchoption/launchoption.md",
	"compattool":    "local/compattool/compattool.md",
	"account":       "local/account/account.md",
	"shortcut":      "local/shortcut/shortcut.md",
	"ownedgame":     "web/ownedgame/ownedgame.md",
	"wishlistitem":  "web/wishlistitem/wishlistitem.md",
	"friend":        "web/friend/friend.md",
	"achievement":   "web/achievement/achievement.md",
	"profile":       "web/profile/profile.md",
}

func pagePath(singular string) string { return pageLayout[singular] }

// pages collects the documentation of every kind, keyed by where it lives.
func pages() map[string]string {
	out := map[string]string{"index.md": indexPage}
	for path, page := range map[string]string{
		"local/app/app.md":                     app.Page,
		"local/libraryfolder/libraryfolder.md": libraryfolder.Page,
		"local/launchoption/launchoption.md":   launchoption.Page,
		"local/compattool/compattool.md":       compattool.Page,
		"local/account/account.md":             account.Page,
		"local/shortcut/shortcut.md":           shortcut.Page,
		"web/ownedgame/ownedgame.md":           ownedgame.Page,
		"web/wishlistitem/wishlistitem.md":     wishlistitem.Page,
		"web/friend/friend.md":                 friend.Page,
		"web/achievement/achievement.md":       achievement.Page,
		"web/profile/profile.md":               profile.Page,
	} {
		out[path] = page
	}
	return out
}
