---
subcategory: Web API
verbs: [get, describe]
---

# Friend

Someone on the account's friend list, from `ISteamUser/GetFriendList`.
Read-only.

## Example

```console
$ whoctl get steam/fr
STEAMID             PERSONA   STATE     PLAYING
76561197960287930   Gabe      online    Portal 2
76561197960287931   Alyx      offline   -
```

## Spec

<!-- whoctl:begin spec -->
_None._
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `steamId64` | string | The friend's 64-bit SteamID, same as metadata.name. |
| `personaName` | string | The friend's display name. Example: `Gabe`. |
| `state` | string | Online, offline, away, busy and so on. Example: `online`. |
| `playing` | string | What the friend is playing right now, when their profile shows it. |
| `friendSince` | integer | Unix time the friendship started. |
| `profileUrl` | string | Link to the friend's community profile. |
| `country` | string | The country on the friend's profile, when it is public. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `STEAMID` | always |
| `PERSONA` | always |
| `STATE` | always |
| `PLAYING` | always |
| `SINCE` | `-o wide` |
| `COUNTRY` | `-o wide` |
<!-- whoctl:end columns -->

**Read-only, because the API is.** Valve publishes no endpoint that changes
this, and whoctl will not drive the store's web session to pretend otherwise.
`apply` and `delete` refuse with that explanation rather than failing obscurely.

**Needs an API key.** The Steam client's local session does not authenticate the
Web API. Set `STEAM_API_KEY` from <https://steamcommunity.com/dev/apikey>; the
account defaults to whoever last signed in locally, and `STEAM_ID` overrides it.

**Two calls, one listing.** The friend list carries only SteamIDs, so names and
presence come from a second call to `GetPlayerSummaries`, batched a hundred at a
time as the API requires.

**A friend with a private profile shows an id and little else.** Their presence
and name are theirs to publish, and an empty column is the honest rendering.
