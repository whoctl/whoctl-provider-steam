---
subcategory: Local
verbs: [get, describe, apply, edit, delete]
---

# Account

A Steam account signed in on this machine, from `config/loginusers.vdf`. The
object's name is the login name, and the persona name or either form of the
SteamID resolve to it too.

## Example

```yaml
apiVersion: steam.whoctl.io/v1alpha1
kind: Account
metadata:
  name: gaben
spec:
  autoLogin: true
  offlineMode: false
```

```console
$ whoctl get steam/acct -o wide
NAME    PERSONA   AUTOLOGIN   RECENT   STEAMID64           ACCOUNTID   OFFLINE
gaben   Gabe      true        true     76561197960287930   22202       false
alt     Alt       false       false    76561197960287931   22203       true
```

## Spec

<!-- whoctl:begin spec -->
| Field | Type | Notes | Description |
| --- | --- | --- | --- |
| `autoLogin` | boolean | optional | Whether the client signs this account in automatically at startup. Only one account can have it. |
| `rememberPassword` | boolean | optional | Whether the client keeps this account's credentials for the next launch. |
| `offlineMode` | boolean | optional | Whether the client starts this account in offline mode. |
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `accountName` | string | The login name, same as metadata.name. Example: `gaben`. |
| `personaName` | string | The display name shown to friends. Example: `Gabe`. |
| `steamId64` | string | The account's 64-bit SteamID. Example: `76561197960287930`. |
| `accountId` | string | The 32-bit account id, which names the directory under userdata. Example: `22202`. |
| `autoLogin` | boolean | Whether this account signs in automatically. |
| `rememberPassword` | boolean | Whether credentials are kept between launches. |
| `offlineMode` | boolean | Whether the client starts offline for this account. |
| `mostRecent` | boolean | Whether this is the account the client last used, which is the one the per-user kinds act on by default. |
| `lastLogin` | integer | Unix time of the last sign-in. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `NAME` | always |
| `PERSONA` | always |
| `AUTOLOGIN` | always |
| `RECENT` | always |
| `STEAMID64` | `-o wide` |
| `ACCOUNTID` | `-o wide` |
| `OFFLINE` | `-o wide` |
<!-- whoctl:end columns -->

## Behaviour

**Nothing that authenticates is read.** The spec covers the three startup flags
and nothing else. The refresh tokens newer clients store in the same file, and
everything in `local.vdf`, are never parsed and can never reach an exported
manifest — the same rule the linux provider applies to password hashes.

`rememberPassword` is a flag about whether the client keeps credentials, not a
credential.

**`autoLogin` is exclusive.** Steam signs exactly one account in automatically,
so turning it on for one account turns it off for the others in the same apply,
rather than leaving the client to pick arbitrarily.

**`apply` never signs in.** Authentication is not something whoctl takes part
in; an account that has never signed in on this machine has no entry to manage,
and the error says so.

**`delete` forgets a login.** The entry goes so the client stops offering the
name at its sign-in prompt. Nothing on Valve's side is touched.

**`status.accountId` is what names the userdata directory.** It is the 32-bit
form of the SteamID, and the directory the per-account kinds read from.
