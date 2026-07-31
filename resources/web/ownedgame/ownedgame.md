---
subcategory: Web API
verbs: [get, describe]
---

# OwnedGame

A game the account owns, from `IPlayerService/GetOwnedGames`. Read-only.

## Example

```console
$ whoctl get steam/owned
APPID   NAME                 PLAYTIME   INSTALLED
620     Portal 2             42.5h      true
570     Dota 2               11.2h      false
440     Team Fortress 2      3.1h       false

$ whoctl get steam/owned 620 -o yaml
apiVersion: steam.whoctl.io/v1alpha1
kind: OwnedGame
metadata:
  name: "620"
status:
  appId: "620"
  name: Portal 2
  playtimeMinutes: 2550
  installed: true
```

## Spec

<!-- whoctl:begin spec -->
_None._
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `appId` | string | Steam's numeric id for the game. Example: `620`. |
| `name` | string | The game's title. Example: `Portal 2`. |
| `playtimeMinutes` | integer | Total playtime in minutes, across every platform. |
| `playtime2WeeksMinutes` | integer | Playtime in the last two weeks, in minutes. |
| `playtimeLinuxMinutes` | integer | Playtime on Linux, in minutes. |
| `playtimeWindowsMinutes` | integer | Playtime on Windows, in minutes. |
| `playtimeDeckMinutes` | integer | Playtime on a Steam Deck, in minutes. |
| `lastPlayed` | integer | Unix time the game was last played. |
| `installed` | boolean | Whether the game is also installed on this machine, which is the local half of the provider answering. |
| `hasStats` | boolean | Whether the game reports achievements and stats. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `APPID` | always |
| `NAME` | always |
| `PLAYTIME` | always |
| `INSTALLED` | always |
| `2WEEKS` | `-o wide` |
| `LINUX` | `-o wide` |
<!-- whoctl:end columns -->

**Read-only, because the API is.** Valve publishes no endpoint that changes
this, and whoctl will not drive the store's web session to pretend otherwise.
`apply` and `delete` refuse with that explanation rather than failing obscurely.

**Needs an API key.** The Steam client's local session does not authenticate the
Web API. Set `STEAM_API_KEY` from <https://steamcommunity.com/dev/apikey>; the
account defaults to whoever last signed in locally, and `STEAM_ID` overrides it.

**`installed` is the local half answering.** The Web API does not know what is
on this machine; the column comes from the appmanifests, so a library listing
shows at a glance what is owned but not downloaded.

**Sorted by playtime.** A library of a thousand titles is unreadable in the
order Steam happens to return it.

**A private profile reads as an error, not as an empty library.** Game details
have to be public for this endpoint to answer, and the message says so.
