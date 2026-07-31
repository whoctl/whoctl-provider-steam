---
subcategory: Local
verbs: [get, describe, apply, edit]
---

# App

A game or tool installed in a Steam library, read from its `appmanifest_*.acf`.
The object's name is the app id, though an installed game also resolves by its
display name when that is unambiguous.

## Example

```yaml
apiVersion: steam.whoctl.io/v1alpha1
kind: App
metadata:
  name: "2200"
spec:
  autoUpdate: onLaunch
```

```console
$ whoctl get steam/apps
APPID     NAME                                        STATE            SIZE
2200      Quake III Arena                             fullyInstalled   489.6M
268910    Cuphead                                     fullyInstalled   3.8G
292120    FINAL FANTASY XIII                          fullyInstalled   57.6G

$ whoctl get steam/app "Quake III Arena" -o wide
APPID   NAME              STATE            SIZE     AUTOUPDATE   LIBRARY                          INSTALLDIR
2200    Quake III Arena   fullyInstalled   489.6M   always       /home/user/.local/share/Steam    Quake 3 Arena
```

## Spec

<!-- whoctl:begin spec -->
| Field | Type | Notes | Description |
| --- | --- | --- | --- |
| `autoUpdate` | string | optional | When Steam updates the app: always, onLaunch or highPriority. Example: `always`. |
| `allowOtherDownloadsWhileRunning` | boolean | optional | Whether other downloads may continue while this app is running. |
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `appId` | string | Steam's numeric id for the app, same as metadata.name. Example: `2200`. |
| `name` | string | The app's display name as Steam recorded it. Example: `Quake III Arena`. |
| `state` | string | What the StateFlags field says: installed, updateRequired, downloading and so on. Example: `installed`. |
| `stateFlags` | integer | The raw StateFlags bitmask, kept because Steam adds flags faster than they can be named. Example: `4`. |
| `installDir` | string | The directory under the library's steamapps/common. Example: `Quake 3 Arena`. |
| `library` | string | The Steam library the app is installed in. Example: `/home/user/.local/share/Steam`. |
| `sizeOnDisk` | integer | Installed size in bytes. |
| `buildId` | string | The build currently installed. Example: `242358`. |
| `lastUpdated` | integer | Unix time of the last update. |
| `lastPlayed` | integer | Unix time the app was last launched, or zero if never. |
| `autoUpdate` | string | The app's update policy. |
| `manifest` | string | The appmanifest file the state was read from. Example: `/home/user/.local/share/Steam/steamapps/appmanifest_2200.acf`. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `APPID` | always |
| `NAME` | always |
| `STATE` | always |
| `SIZE` | always |
| `AUTOUPDATE` | `-o wide` |
| `LIBRARY` | `-o wide` |
| `INSTALLDIR` | `-o wide` |
<!-- whoctl:end columns -->

## Behaviour

**`apply` never installs, and `delete` never uninstalls.** The spec covers the
two settings the manifest owns — the rest of an appmanifest is a download's
bookkeeping — and installing is an asynchronous transfer the client performs
after a human confirms a dialog. That is not a desired state anything can
converge on, so the honest answer is a refusal:

```console
$ whoctl delete steam/app 2200
error: apps cannot be uninstalled by whoctl: deleting an appmanifest would strand the
game's files and leave Steam's records inconsistent; uninstall from the client
```

**`status.state` decomposes StateFlags.** Steam packs the install state into a
bitmask it has never documented; `4` is `fullyInstalled` and `1026` is
`updateRequired,updateStarted`. The raw number stays in `status.stateFlags`,
because Steam adds flags faster than they can be named.

**Every library is walked.** Games live wherever they were installed, so a
listing reads the `steamapps` directory of each library in
`libraryfolders.vdf`. A library that is configured but not currently mounted —
an external drive, unplugged — contributes nothing rather than failing the read.

**Unmodelled keys survive.** Changing `autoUpdate` rewrites that one field; the
depot manifests, sizes and staging counters that make up most of the file are
copied through untouched.

See [LaunchOption](launchoption.md) for how a game is launched and
[CompatTool](compattool.md) for what it runs under.
