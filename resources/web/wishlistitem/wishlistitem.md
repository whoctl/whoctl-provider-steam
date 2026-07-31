---
subcategory: Web API
verbs: [get, describe]
---

# WishlistItem

A game on the account's wishlist, from `IWishlistService/GetWishlist`.
Read-only.

## Example

```console
$ whoctl get steam/wish
APPID     PRIORITY   OWNED
1245620   1          false
367520    2          true
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
| `priority` | integer | The account's own ordering of the wishlist, 1 being the top. |
| `dateAdded` | integer | Unix time the game was added to the wishlist. |
| `owned` | boolean | Whether the account already owns the game, which happens when a wishlisted item is bought but not removed. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `APPID` | always |
| `PRIORITY` | always |
| `OWNED` | always |
| `ADDED` | `-o wide` |
<!-- whoctl:end columns -->

**Read-only, because the API is.** Valve publishes no endpoint that changes
this, and whoctl will not drive the store's web session to pretend otherwise.
`apply` and `delete` refuse with that explanation rather than failing obscurely.

**Needs an API key.** The Steam client's local session does not authenticate the
Web API. Set `STEAM_API_KEY` from <https://steamcommunity.com/dev/apikey>; the
account defaults to whoever last signed in locally, and `STEAM_ID` overrides it.

**`owned` catches the usual wishlist drift.** A game bought without being
removed from the wishlist stays on it; the column is filled by cross-checking
the owned library, so those entries are visible rather than confusing.

**Sorted by the account's own priority**, which is the order the store shows.
