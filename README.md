# whoctl-provider-steam

The steam provider for [whoctl](https://github.com/whoctl/whoctl): a local Steam
installation, and the account behind it read from Steam's Web API.

```console
$ whoctl get steam/apps
$ whoctl get steam/achievements 620
$ whoctl apply -f launchoptions.yaml
```

Installed on first use; nothing to do by hand.

## Two halves, authenticated differently, and only one writable

The **local half** parses and rewrites Steam's KeyValues files and needs nothing
but an installation: `App`, `LibraryFolder`, `LaunchOption`, `CompatTool`,
`Account`, `Shortcut`.

The **web half** needs an API key — *the client's local session does not
authenticate the Web API*, which is the single most common wrong assumption
about it:

```sh
export STEAM_API_KEY=…    # from steamcommunity.com/dev/apikey
export STEAM_ID=…         # optional: defaults to whoever last signed in locally
```

Everything the Web API exposes about a player is read-only, so `OwnedGame`,
`WishlistItem`, `Friend`, `Achievement` and `Profile` refuse `apply` and
`delete` with a reason rather than pretending.

## Mutations refuse while Steam is running

The client holds its configuration in memory and rewrites these files on exit,
so a change written underneath it is discarded at best and interleaved at worst.
There is no lock to take, so the provider refuses:

```console
$ whoctl apply -f launchoptions.yaml
error: Steam is running: quit the client before changing launch options,
or it will overwrite the change on exit
```

## Layout

| Path | Role |
| --- | --- |
| `resources/<name>` | One directory per kind: its handler, its tests, its page and its example. Eleven of them. |
| `internal/provider` | The state every kind works from: where the installation is, whether the client is running, the Web API client. |
| `internal/steam` | Assembly: the provider, its overview page, and the one list of kinds. |
| `internal/webkind` | What the five Web API kinds share: the read-only refusal and its reason. |
| `internal/vdf` | Valve Data Format: the text and binary KeyValues parsers and writers. |
| `internal/steamapi` | Client for Steam's public Web API. |
| `internal/steamtest` | The fixture every resource package tests against. |
| `testdata/steam` | The fixture installation they all read. |

## Never touch a real Steam installation from a test

A developer running this suite has a live Steam install one directory away — see
`CLAUDE.md`.
