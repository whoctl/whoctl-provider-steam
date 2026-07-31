package main

import (
	"context"
	"strings"
	"testing"

	"github.com/whoctl/whoctl-sdk-go/core"

	"github.com/whoctl/whoctl-provider-steam/internal/steam"
)

// The Web API half is read-only, and that is a property of every kind in it
// rather than of any one. These assertions live here, over the assembled
// provider, because that is the smallest thing that has all five.
//
// fixtureRoot is a Steam installation that is not the developer's.
const fixtureRoot = "testdata/steam"

func handlerFor(t *testing.T, p core.Provider, kind string) core.Handler {
	t.Helper()
	for _, h := range p.Handlers() {
		if h.Type().Kind == kind {
			return h
		}
	}
	t.Fatalf("no handler for kind %q", kind)
	return nil
}

func TestWebKindsExplainTheirCredentials(t *testing.T) {
	p := steam.New(steam.Options{SteamRoot: fixtureRoot})
	// No key is configured, and the fixture's local login cannot supply one:
	// the client session does not authenticate the Web API.
	_, err := handlerFor(t, p, "OwnedGame").List(context.Background())
	if err == nil {
		t.Fatal("a web kind with no key must fail rather than return nothing")
	}
	for _, want := range []string{"dev/apikey", "STEAM_API_KEY", "signed in to the Steam client is not enough"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v\nwant it to mention %q", err, want)
		}
	}
}

func TestWebKindsRefuseMutation(t *testing.T) {
	p := steam.New(steam.Options{SteamRoot: fixtureRoot})
	for _, kind := range []string{"OwnedGame", "WishlistItem", "Friend", "Achievement", "Profile"} {
		h := handlerFor(t, p, kind)
		if _, err := h.Apply(context.Background(), core.Object{Metadata: core.Metadata{Name: "x"}}); err == nil {
			t.Errorf("%s accepted an apply", kind)
		} else if !strings.Contains(err.Error(), "read-only") {
			t.Errorf("%s: err = %v, want it to say read-only", kind, err)
		}
		if err := h.Delete(context.Background(), "x"); err == nil {
			t.Errorf("%s accepted a delete", kind)
		}
	}
}

// The refusal has to name the kind and say why. An empty reason still contains
// the words "read-only" while telling the user nothing.
func TestWebKindsSayWhichKindIsReadOnlyAndWhy(t *testing.T) {
	h := handlerFor(t, steam.New(steam.Options{SteamRoot: fixtureRoot}), "Achievement")
	_, err := h.Apply(context.Background(), core.Object{Metadata: core.Metadata{Name: "x"}})
	if err == nil {
		t.Fatal("Achievement accepted an apply")
	}
	for _, want := range []string{"Achievement", "no endpoint"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v\nwant it to mention %q", err, want)
		}
	}
}
