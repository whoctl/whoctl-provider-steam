---
subcategory: Local
---

# Shortcut

A non-Steam game added to the library, from the account's `shortcuts.vdf`. The
object's name is the shortcut's title.

This is the one Steam resource with a full lifecycle. Shortcuts are entirely the
user's — Steam only stores them, and nothing on Valve's side has to agree — so
unlike `App`, this kind really does create and delete.

## Example

```yaml
apiVersion: steam.whoctl.io/v1alpha1
kind: Shortcut
metadata:
  name: Lutris
spec:
  exe: /usr/bin/lutris
  startDir: /usr/bin/
  tags: [Emulators]
  allowOverlay: true
```

```console
$ whoctl apply -f lutris.yaml
steam/shortcut/Lutris created

$ whoctl get steam/nonsteam
NAME     EXE                TAGS        HIDDEN
Lutris   /usr/bin/lutris    Emulators   false
```

## Spec

<!-- whoctl:begin spec -->
| Field | Type | Notes | Description |
| --- | --- | --- | --- |
| `exe` | string | **required** | The program to run. Quote it if the path contains spaces, the way Steam itself writes it. Example: `/usr/bin/lutris`. |
| `startDir` | string | optional | Working directory for the program. Defaults to the executable's own directory. Example: `/usr/bin/`. |
| `launchOptions` | string | optional | Arguments passed to the program. Example: `rungame/rom`. |
| `icon` | string | optional | Path to an icon file. |
| `tags` | list of string | optional | Steam categories the shortcut appears under. Example: `Emulators`. |
| `hidden` | boolean | optional | Whether the shortcut is hidden from the library. |
| `allowOverlay` | boolean | optional | Whether the Steam overlay is enabled for it. |
| `allowDesktopConfig` | boolean | optional | Whether the desktop controller configuration applies. |
| `account` | string | optional | Which signed-in account to add the shortcut for. Defaults to the one the client used last. Example: `gaben`. |
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `appName` | string | The shortcut's name, same as metadata.name. Example: `Lutris`. |
| `appId` | string | The id Steam assigns the shortcut, which is derived from the executable and the name. |
| `exe` | string | The program that runs. |
| `startDir` | string | Working directory for the program. |
| `launchOptions` | string | Arguments passed to the program. |
| `icon` | string | Path to the icon file. |
| `tags` | list of string | Steam categories the shortcut appears under. |
| `hidden` | boolean | Whether the shortcut is hidden from the library. |
| `allowOverlay` | boolean | Whether the Steam overlay is enabled. |
| `allowDesktopConfig` | boolean | Whether the desktop controller configuration applies. |
| `lastPlayed` | integer | Unix time the shortcut was last launched. |
| `account` | string | The account the shortcut belongs to. |
| `file` | string | The shortcuts.vdf the entry was read from. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `NAME` | always |
| `EXE` | always |
| `TAGS` | always |
| `HIDDEN` | always |
| `APPID` | `-o wide` |
| `STARTDIR` | `-o wide` |
| `ACCOUNT` | `-o wide` |
<!-- whoctl:end columns -->

## Behaviour

**`shortcuts.vdf` is binary.** Steam writes this one in its binary KeyValues
encoding rather than the text form the rest of its configuration uses. whoctl
reads and writes it directly; the fields Steam stores as 32-bit integers are
written back as integers, because a shortcut whose `appid` is a string is
silently discarded by the client.

**The app id is derived, not chosen.** Steam computes it from the executable and
the name, with the top bit set so it can never collide with a real app id.
whoctl reproduces that calculation, which is what lets artwork and controller
configuration find the shortcut.

**Omitted fields are left alone.** A shortcut the client created keeps the flags
the client set on it; only what the manifest names is reconciled.

**Deleting renumbers the rest.** Steam keys shortcuts by consecutive index, so
the survivors are renumbered rather than left with a hole the client would
misread.

**Per account, and Steam must be closed** — the same two rules that apply to
[LaunchOption](launchoption.md).
