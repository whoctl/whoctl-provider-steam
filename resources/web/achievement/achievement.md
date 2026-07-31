---
subcategory: Web API
verbs: [get, describe]
---

# Achievement

An achievement for one game, from `ISteamUserStats/GetPlayerAchievements`.
Read-only.

## Example

The argument is the game, not the achievement — an app id, or the name of a
game installed on this machine:

```console
$ whoctl get steam/achievements 620
NAME                         DISPLAY               UNLOCKED
ACH_SURVIVE_CONTAINER_RIDE   Bridge Over Troubl…   yes
ACH_BREAK_TVS                Vidiot                no

$ whoctl describe steam/ach "Portal 2"
Name:        ACH_SURVIVE_CONTAINER_RIDE
Unlocked:    true
Description: Survive the container ride
```

## Spec

<!-- whoctl:begin spec -->
_None._
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `apiName` | string | The achievement's internal name, same as metadata.name. Example: `ACH_WIN_ONE_GAME`. |
| `displayName` | string | The name shown in the client. |
| `description` | string | What the achievement asks for. Hidden achievements have none until unlocked. |
| `unlocked` | boolean | Whether the account has it. |
| `unlockTime` | integer | Unix time it was unlocked. |
| `appId` | string | The game the achievement belongs to. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `NAME` | always |
| `DISPLAY` | always |
| `UNLOCKED` | always |
| `APPID` | `-o wide` |
| `DESCRIPTION` | `-o wide` |
<!-- whoctl:end columns -->

**Read-only, because the API is.** Valve publishes no endpoint that changes
this, and whoctl will not drive the store's web session to pretend otherwise.
`apply` and `delete` refuse with that explanation rather than failing obscurely.

**Needs an API key.** The Steam client's local session does not authenticate the
Web API. Set `STEAM_API_KEY` from <https://steamcommunity.com/dev/apikey>; the
account defaults to whoever last signed in locally, and `STEAM_ID` overrides it.

**Achievements are per game, and there is no listing across all of them.** An
account owning a thousand titles would mean a thousand API calls, and Valve rate
limits for a reason. Naming a game is required, and the error says so rather
than quietly fetching for hours.

**The game is the argument, because an achievement is not addressable alone.**
`ACH_WIN_ONE_GAME` is unique inside its game and nowhere else, so there is no
name that would identify one achievement on its own. `get` and `describe` read
the argument as the game and expand it into that game's achievements — several
objects from one argument, which is what `core.ScopedLister` exists for. Several
games at once work the same way: `whoctl get steam/ach 620 570`.

**A game with no achievements reports why.** "Requested app has no stats" is the
normal answer for most titles, and reads better than an empty list with no
explanation.

**Hidden achievements have no description until unlocked**, which is Steam's
behaviour showing through rather than a gap in the data.
