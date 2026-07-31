---
subcategory: Web API
verbs: [get, describe]
---

# Profile

The account's public Steam profile, from `ISteamUser/GetPlayerSummaries`.
Read-only.

## Example

```console
$ whoctl get steam/me
STEAMID             PERSONA   STATE    VISIBILITY   LEVEL
76561197960287930   Gabe      online   public       42
```

Any SteamID can be named, since a profile is public information:

```sh
whoctl get steam/profile 76561197960287931
```

## Spec

<!-- whoctl:begin spec -->
_None._
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `steamId64` | string | The 64-bit SteamID. |
| `personaName` | string | The display name. Example: `Gabe`. |
| `realName` | string | The real name on the profile, when it is public. |
| `state` | string | Online, offline, away and so on. |
| `visibility` | string | Whether the profile is public or private, which decides what the rest of these kinds can read. Example: `public`. |
| `playing` | string | What the account is playing right now. |
| `country` | string | The country on the profile. |
| `profileUrl` | string | Link to the community profile. |
| `level` | integer | The Steam community level. |
| `created` | integer | Unix time the account was created. |
| `lastLogoff` | integer | Unix time of the last sign-out. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `STEAMID` | always |
| `PERSONA` | always |
| `STATE` | always |
| `VISIBILITY` | always |
| `LEVEL` | always |
| `PLAYING` | `-o wide` |
| `URL` | `-o wide` |
<!-- whoctl:end columns -->

**Read-only, because the API is.** Valve publishes no endpoint that changes
this, and whoctl will not drive the store's web session to pretend otherwise.
`apply` and `delete` refuse with that explanation rather than failing obscurely.

**Needs an API key.** The Steam client's local session does not authenticate the
Web API. Set `STEAM_API_KEY` from <https://steamcommunity.com/dev/apikey>; the
account defaults to whoever last signed in locally, and `STEAM_ID` overrides it.

**`visibility` explains the rest of the provider.** A private profile is why
`OwnedGame`, `WishlistItem` and `Achievement` return nothing useful, so it is
worth checking first when they do.

**`level` is only filled for the configured account**, because the endpoint that
reports it answers for that account alone.
