# whoctl-provider-steam

A local Steam installation and the account behind it. A binary speaking whoctl's
protocol over stdio, built on `github.com/whoctl/whoctl-sdk-go`.

**Never touch a real Steam installation from a test.** A developer running this
suite has a live install one directory away. `steamtest.Fixture` reads
`testdata/steam`; every mutating test goes through `steamtest.Writable`, which
copies that tree into `t.TempDir()` first. Nothing in the suite may resolve the
real Steam root, which is why detection is never exercised against `$HOME`.

There is no container suite here, and that is not an oversight: with the fixture
rule above, `go test ./...` is safe on the workstation. The delete rule in the
workspace `CLAUDE.md` still applies, and `Shortcut` is the kind it is about —
the one with a real delete, because shortcuts are entirely the user's.

## Layout: two halves

A kind lives in `resources/<half>/<kind>/` with its spec, its handler, its tests
and its page. `resources/local` reads and rewrites Steam's own KeyValues files;
`resources/web` asks the public Web API. What a half shares sits at its root, so
`internal/` holds no domain logic: the assembly, the fixtures, and a `provider`
that is little more than the installation root.

**Three imports cross from `web` to `local`, and they are the point.** The
halves inform each other on purpose:

- `OwnedGame.installed` is the local half answering.
- `Achievement` accepts the name of a game installed on this machine.
- The Web API client asks which account signed in last, which is in
  `loginusers.vdf`.

They were invisible while everything lived in one package. Keep them visible.

## Decisions somebody would otherwise undo

**The client's local session does not authenticate the Web API.** This is the
single most common wrong assumption about Steam. The web half needs a key from
steamcommunity.com/dev/apikey; being signed in buys nothing.

Everything the Web API exposes about a player is read-only — Valve publishes no
endpoint that buys a game, edits a wishlist or adds a friend — so those kinds
refuse `apply` and `delete` with a reason rather than pretending.

**Mutations refuse while Steam is running.** The client holds its configuration
in memory and rewrites these files on exit, so a change written underneath it is
discarded at best and interleaved at worst. There is no lock to take. The guard
checks `steam.pid` *and* that the process exists, because a stale pid file from
an unclean exit would otherwise lock the user out of their own config. Reads are
always allowed.

That existence check has to be right in one direction in particular: a false
"not running" lets a write through while the client is up. It is per-platform
(`process_unix.go`, `process_windows.go`) rather than reading `/proc`, which
answers only on Linux, and EPERM counts as running — a client owned by another
user is still a client.

**Nothing that authenticates is ever read.** `Account` covers the startup flags
in `loginusers.vdf` and stops there; the refresh tokens in that file and
everything in `local.vdf` are never parsed, so they cannot reach an exported
manifest.

**`shortcuts.vdf` is binary KeyValues.** Same tree, different encoding. The
fields Steam stores as int32 must be written back as int32 — a shortcut whose
`appid` is a string is silently discarded by the client.

**An achievement is not addressable alone.** Its api name is unique inside its
game and nowhere else, so `get` and `describe` read the argument as the game and
expand it. Listing every achievement of every owned game would be one API call
per title, and Valve rate limits for a reason.

**Configuration comes from the environment, never from the CLI.**
`STEAM_API_KEY`, `STEAM_ID` and `WHOCTL_STEAM_ROOT` are read by this process. A
steam-specific flag on `whoctl get` would be the abstraction leaking.

## Documentation

The prose is written by hand in each kind's `<kind>.md`; the tables come from
`doc` tags and are injected between HTML-comment markers. `make docs` writes the
bundle a release publishes, and `TestConformance` fails on a field with no doc
tag, a kind with no page, or stale tables. Do not "fix" it by deleting the check.
