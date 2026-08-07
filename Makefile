# whoctl-provider-steam
#
# Mutating verbs create and delete real accounts, so every target that runs
# them does it inside a throwaway Alpine container. Nothing here changes the
# workstation.

# Overridable on the command line: make sandbox DISTRO=debian
#
# One distro per package manager: alpine=apk, debian=apt, fedora=dnf, arch=pacman.
# TOOLSET only means something on alpine, the one distro that ships BusyBox's
# account applets instead of shadow-utils.
VERSION          ?= dev

export VERSION

.DEFAULT_GOAL := help

## build: build the provider binary
.PHONY: build
build:
	@mkdir -p bin
	@CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(VERSION)" \
		-o bin/whoctl-provider-steam .
	@echo "built bin/whoctl-provider-steam ($(VERSION))"

## test: the whole suite. Safe on the host: it reads a fixture installation.
.PHONY: test
test:
	@go test ./...

## docs: write the documentation bundle a release publishes
.PHONY: docs
docs:
	@go run . --docs-bundle > bundle.json
	@echo "wrote bundle.json"

## fmt: format and vet
.PHONY: fmt
fmt:
	@gofmt -w .
	@go vet ./...

## clean: remove build output
.PHONY: clean
clean:
	@rm -rf bin

## standalone: build and test without the workspace, the way a consumer does
#
# The check lives in whoctl, beside the container harness and for the same
# reason: it is about how a module is consumed, not about what this one manages.
.PHONY: standalone
standalone:
	@../whoctl/scripts/standalone.sh

## help: list the available targets
.PHONY: help
help:
	@echo "whoctl-provider-steam targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  %-9s %s\n", $$1, $$2}'
