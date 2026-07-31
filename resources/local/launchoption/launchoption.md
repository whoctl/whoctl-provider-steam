---
subcategory: Local
---

# LaunchOption

The launch command for one app, from the account's `localconfig.vdf`. This is
the field behind "Launch Options" in the client's properties dialog, where
`%command%` stands for the game's own executable.

## Example

```yaml
apiVersion: steam.whoctl.io/v1alpha1
kind: LaunchOption
metadata:
  name: "2486820"
spec:
  options: mangohud %command%
```

```console
$ whoctl get steam/launchopt
APPID     APP                         OPTIONS
2486820   Sonic Racing: CrossWorlds   mangohud %command%

$ whoctl apply -f gamemode.yaml
steam/launchoption/620 created
```

The name may be an app id or an installed game's title:

```sh
whoctl get steam/launchopt "Portal 2"
```

## Spec

<!-- whoctl:begin spec -->
| Field | Type | Notes | Description |
| --- | --- | --- | --- |
| `options` | string | **required** | The launch command. %command% is replaced by the game's own executable, so a wrapper goes before it and extra arguments after. Example: `mangohud %command%`. |
| `account` | string | optional | Which signed-in account to set this for. Defaults to the one the client used last. Example: `gaben`. |
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `appId` | string | The app the options belong to, same as metadata.name. Example: `2200`. |
| `appName` | string | The app's display name, when it is installed on this machine. Example: `Quake III Arena`. |
| `options` | string | The launch command currently set. Example: `mangohud %command%`. |
| `account` | string | The account whose configuration this came from. Example: `gaben`. |
| `file` | string | The localconfig.vdf the value was read from. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `APPID` | always |
| `APP` | always |
| `OPTIONS` | always |
| `ACCOUNT` | `-o wide` |
<!-- whoctl:end columns -->

## Behaviour

**Only apps with options set are listed.** `localconfig.vdf` carries an entry
for every app the account has ever touched, and listing hundreds of empty
strings would bury the handful that mean something.

**Options can be set before a game is ever launched.** The app's block is
created if it is not there, which is exactly when someone wants to configure a
wrapper in advance.

**`delete` clears the command, it does not remove the app's block.** Steam keeps
playtime and other bookkeeping in that block which has nothing to do with
whoctl.

**Per account.** The value belongs to one signed-in account; without
`spec.account` it is whichever the client used last.

**Steam must be closed.** The client rewrites `localconfig.vdf` on exit, so a
change made underneath it is discarded. `apply` refuses rather than losing the
edit silently.
