package main

import (
	"testing"

	"github.com/whoctl/whoctl-sdk-go/providertest"

	"github.com/whoctl/whoctl-provider-steam/internal/steam"
)

// The whole of this provider's contract with whoctl, in one test. It reads the
// resource types and the embedded pages; it never resolves a Steam root, so it
// cannot reach the developer's own installation.
func TestConformance(t *testing.T) {
	providertest.Conformance(t, steam.New(steam.Options{}), providertest.Options{SourceRoot: "."})
}
