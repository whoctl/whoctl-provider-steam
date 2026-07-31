// Package webkind is what every kind backed by Steam's Web API shares.
//
// They are all read-only, and that is a property of the API rather than a
// decision taken here: Valve publishes no endpoint that buys a game, edits a
// wishlist, adds a friend or changes a profile.
package web

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/whoctl/whoctl-sdk-go/core"

	"github.com/whoctl/whoctl-provider-steam/internal/provider"
	"github.com/whoctl/whoctl-provider-steam/resources/local"
	"github.com/whoctl/whoctl-provider-steam/resources/web/steamapi"
)

// The kinds backed by Steam's Web API.
//
// Every one of them is read-only, and that is a property of the API rather than
// a decision taken here: Valve publishes no endpoint that buys a game, edits a
// wishlist, adds a friend or changes a profile. Rather than accept an `apply`
// that could never converge, these implement the read verbs and refuse the rest
// with an explanation — the same shape `Service` uses for a verb that does not
// fit, one step further.

// readOnly implements the mutating half of core.Handler for a kind that has no
// mutating half, so each kind states its reason once and shares the rest.
// ReadOnly implements the mutating half of core.Handler for a kind that has no
// mutating half, so each kind states its reason once and shares the rest.
type ReadOnly struct {
	Kind   string
	Reason string
}

func (r ReadOnly) Apply(ctx context.Context, obj core.Object) (core.Result, error) {
	return core.Result{}, r.refuse()
}

func (r ReadOnly) Delete(ctx context.Context, name string) error {
	return r.refuse()
}

func (r ReadOnly) refuse() error {
	return core.Unsupportedf("%s is read-only: %s", r.Kind, r.Reason)
}

// NoWriteAPI is the reason every one of them gives.
const NoWriteAPI = "Steam's Web API publishes no endpoint to change it, and whoctl will not drive the store's web session to pretend otherwise"

// PersonaStates are the numbers ISteamUser returns for a profile's presence.
var PersonaStates = []string{"offline", "online", "busy", "away", "snooze", "lookingToTrade", "lookingToPlay"}

// PersonaState names what Steam reports as a number.
func PersonaState(v int) string {
	if v >= 0 && v < len(PersonaStates) {
		return PersonaStates[v]
	}
	return fmt.Sprintf("unknown(%d)", v)
}

// API returns the Web API client, or an error naming what is missing.
// API returns the Web API client, or an error naming what is missing.
//
// It lives in the web family and asks the local half which account signed in
// last, because that is where loginusers.vdf is read — the same arrow as
// OwnedGame's installed flag, in the other direction.
func API(p *provider.Provider) (*steamapi.Client, error) {
	clientOnce.Do(func() {
		key := provider.FirstNonEmpty(p.Opts.APIKey, os.Getenv("STEAM_API_KEY"), os.Getenv("WHOCTL_STEAM_API_KEY"))
		id := provider.FirstNonEmpty(p.Opts.SteamID, os.Getenv("STEAM_ID"), os.Getenv("WHOCTL_STEAM_ID"))
		if id == "" {
			// The signed-in account is the obvious default, and loginusers.vdf
			// is where the client records it.
			if detected, err := local.MostRecentSteamID(p); err == nil {
				id = detected
			}
		}
		client, clientErr = steamapi.New(key, id)
	})
	return client, clientErr
}

var (
	clientOnce sync.Once
	client     *steamapi.Client
	clientErr  error
)
