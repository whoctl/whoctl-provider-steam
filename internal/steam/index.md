---
title: Overview
---

# Steam provider

The steam provider manages a Steam installation on the machine whoctl runs on,
and reads the account behind it from Steam's Web API. Its resources are
addressed as `steam/<resource>`.

```console
$ whoctl get steam/apps
APPID     NAME                                        STATE            SIZE
2200      Quake III Arena                             fullyInstalled   489.6M
268910    Cuphead                                     fullyInstalled   3.8G
292120    FINAL FANTASY XIII                          fullyInstalled   57.6G

$ whoctl get steam/launchopt
APPID     APP                         OPTIONS
2486820   Sonic Racing: CrossWorlds   mangohud %command%
```

## Two halves, authenticated differently

This is the thing to understand before anything else, because it is the source
of nearly every surprise:

**The local half needs nothing but a Steam installation.** Being signed in to
the client is what fills these files in, and whoctl reads and rewrites them
directly. `App`, `LibraryFolder`, `LaunchOption`, `CompatTool`, `Account` and
`Shortcut` all work offline, with no credentials of any kind.

**The web half needs an API key, and the local sign-in does not provide one.**
The Steam client's session does not authenticate the public Web API; that takes
a key issued at <https://steamcommunity.com/dev/apikey>. `OwnedGame`,
`WishlistItem`, `Friend`, `Achievement` and `Profile` all say so when it is
missing:

```console
$ whoctl get steam/wishlist
error: the Steam Web API needs an API key: being signed in to the Steam client is not enough,
since the client's session does not authenticate this API.
Get a key at https://steamcommunity.com/dev/apikey and set STEAM_API_KEY; the account is
taken from the local login, or set STEAM_ID to override it
```

```sh
export STEAM_API_KEY=…      # from steamcommunity.com/dev/apikey
export STEAM_ID=…           # optional: defaults to whoever last signed in locally
```

## Serving it

A context is one Steam installation, on the machine the server is running on or
in a tree it can open.

```yaml
- name: desktop
  provider: steam
  env:
    # Where the installation is. Without it the provider walks the four
    # standard locations under the server process's own $HOME, which on a
    # server is not where anybody's Steam lives.
    WHOCTL_STEAM_ROOT: /home/alice/.steam/steam
    # The Web API half. In ${...} so the file can be committed and the key
    # cannot — it is the account's key, not the machine's.
    STEAM_API_KEY: ${STEAM_API_KEY}
    STEAM_ID: "76561198000000000"
```

Nothing here is namespaced and nothing answers a pod view: an installed game is
not something running, and saying it is would be the shim telling a client
something it cannot check.

The two halves fail independently and on purpose. A context with no
`STEAM_API_KEY` still serves every local kind; the Web API kinds report the
missing key rather than taking the context down with them.

There is no remote mode: to serve the Steam on another machine, run a whoctl
server there, or give this one a path it can read.

## Everything from the Web API is read-only

Not a decision taken here — a property of the API. Valve publishes no endpoint
that buys a game, edits a wishlist, adds a friend or changes a profile; the
write APIs that exist are for publishers acting on their own titles. Those kinds
therefore implement `get` and `describe`, and refuse `apply` and `delete` with
an explanation rather than pretending:

```console
$ whoctl apply -f wishlist.yaml
error: WishlistItem is read-only: Steam's Web API publishes no endpoint to change it,
and whoctl will not drive the store's web session to pretend otherwise
```

The same reasoning limits two local kinds. `App` reconciles the settings of an
installed game but never installs one, because installing is an asynchronous
download the client performs after a human confirms a dialog — not a state
anything can converge on. `Account` manages how a sign-in behaves but never
signs in.

`Shortcut` is the one kind with a full lifecycle: non-Steam games are entirely
the user's, so they are genuinely created and deleted.

## Mutations refuse while Steam is running

Steam holds its configuration in memory and rewrites these files when it exits.
A change written underneath a running client is discarded at best, and
interleaved with the client's own write at worst. There is no lock to take and
no way to ask the client to reload, so whoctl refuses:

```console
$ whoctl apply -f launchoptions.yaml
error: Steam is running: quit the client before changing launch options,
or it will overwrite the change on exit
```

Reads are always allowed. The check looks for `steam.pid` in the Steam root and
confirms the process is genuinely alive, so the stale pid file an unclean exit
leaves behind does not lock you out of your own configuration.

## How it works

**Everything local is KeyValues.** Steam's configuration is all written in
Valve's own format — `libraryfolders.vdf`, `loginusers.vdf`, `config.vdf`, each
`appmanifest_*.acf`, and the per-account `localconfig.vdf`. whoctl parses them
directly and writes them back through a rename, so a crash cannot leave a
half-written config. `shortcuts.vdf` is the same tree in Valve's binary
encoding, and decodes into the same shape.

**Unmodelled keys survive.** A `localconfig.vdf` holds hundreds of keys whoctl
has no opinion about, and an appmanifest is mostly depot bookkeeping. Changing
one field rewrites that field and copies the rest through untouched — the same
rule the linux provider follows for `/etc/resolv.conf`.

**Nothing that authenticates is ever read.** The account kind reads the startup
flags in `loginusers.vdf` and nothing else; the refresh tokens newer clients
store alongside them, and everything in `local.vdf`, are never parsed and can
never reach an exported manifest.

**Per-account kinds default to the last sign-in.** `LaunchOption`, `Shortcut`
and the Web API kinds act on whichever account the client used most recently,
which is the one a machine almost always means. `spec.account` overrides it.

**Libraries are all walked.** Games live in whichever library they were
installed into, so anything listing apps reads every one; a library recorded but
not currently mounted — an external drive, unplugged — is reported as configured
and not mounted rather than silently omitted.

## Requirements

- A Steam installation. It is looked for at `~/.steam/steam`,
  `~/.local/share/Steam`, the flatpak path, and `~/.steam/root`, in that order.
  `WHOCTL_STEAM_ROOT` points at one directly.
- `--root` prefixes the search, which is how the tests run against fixtures.
- The Web API kinds additionally need `STEAM_API_KEY`, and a profile whose game
  details are public for anything about games.
