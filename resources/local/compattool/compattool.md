---
subcategory: Local
---

# CompatTool

Which Proton or other compatibility tool runs an app, from
`config/config.vdf`. The object's name is the app id, or `default` for the
machine-wide setting Steam stores under the key `0`.

## Example

```yaml
apiVersion: steam.whoctl.io/v1alpha1
kind: CompatTool
metadata:
  name: "620"
spec:
  tool: GE-Proton9-20
```

```console
$ whoctl get steam/proton
APPID   APP                  TOOL                  CUSTOM
0       -                    proton_experimental   false
620     Portal 2             GE-Proton9-20         true

$ whoctl delete steam/proton 620
steam/compattool/620 deleted
```

## Spec

<!-- whoctl:begin spec -->
| Field | Type | Notes | Description |
| --- | --- | --- | --- |
| `tool` | string | **required** | The tool to run the app under, named as Steam names it: a built-in build such as proton_experimental, or the directory name of something in compatibilitytools.d. Example: `proton_experimental`. |
| `config` | string | optional | Steam's extra config word for the mapping, which the client writes as it sees fit. Example: `noesync`. |
| `priority` | string | optional | Steam's priority for the mapping. Left alone when omitted. Example: `250`. |
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `appId` | string | The app the mapping applies to, or 0 for the machine-wide default. Example: `2200`. |
| `appName` | string | The app's display name, when it is installed. |
| `tool` | string | The compatibility tool in force. Example: `proton_experimental`. |
| `config` | string | Steam's extra config word. |
| `priority` | string | Steam's priority for the mapping. |
| `global` | boolean | Whether this is the machine-wide default rather than a per-app override. |
| `custom` | boolean | Whether the tool is a user-installed one under compatibilitytools.d rather than a Valve build. |
| `file` | string | The config.vdf the mapping was read from. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `APPID` | always |
| `APP` | always |
| `TOOL` | always |
| `CUSTOM` | always |
| `PRIORITY` | `-o wide` |
| `CONFIG` | `-o wide` |
<!-- whoctl:end columns -->

## Behaviour

**`default` is the machine-wide mapping.** Steam records it under the app id
`0`, which is addressable either way — `whoctl get steam/proton default` and
`whoctl get steam/proton 0` are the same object.

**A cleared mapping is not a mapping.** When the client's "force a specific
compatibility tool" checkbox is unticked it leaves an entry behind with an empty
name. Those are skipped, because an entry that forces nothing is not a
compatibility tool in force.

**`status.custom` says where the tool came from.** A name matching a directory
under `compatibilitytools.d` is a user-installed build such as GE-Proton;
anything else is one of Valve's.

**`delete` stops forcing a tool**, which returns the app to whatever Steam Play
would have chosen for it.

**Unmodelled keys survive.** `config.vdf` is the client's whole install
configuration; a mapping change rewrites the mapping and copies everything else
through.
