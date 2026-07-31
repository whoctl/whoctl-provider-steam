---
subcategory: Local
---

# LibraryFolder

A directory Steam installs games into, from `config/libraryfolders.vdf`. The
object's name is the path, because that is what identifies a library — Steam
keys them by an index it renumbers whenever one is removed.

## Example

```yaml
apiVersion: steam.whoctl.io/v1alpha1
kind: LibraryFolder
metadata:
  name: /mnt/games/SteamLibrary
spec:
  label: Games SSD
```

```console
$ whoctl get steam/lib
PATH                             LABEL       APPS   MOUNTED
/home/user/.local/share/Steam    -           19     true
/mnt/games/SteamLibrary          Games SSD   7      false
```

## Spec

<!-- whoctl:begin spec -->
| Field | Type | Notes | Description |
| --- | --- | --- | --- |
| `label` | string | optional | The name the client shows for this library. Steam leaves it empty for the default one. Example: `Games SSD`. |
<!-- whoctl:end spec -->

## Status

<!-- whoctl:begin status -->
| Field | Type | Description |
| --- | --- | --- |
| `path` | string | The library directory, same as metadata.name. Example: `/home/user/.local/share/Steam`. |
| `label` | string | The client's name for the library. |
| `index` | string | Steam's position for the library in libraryfolders.vdf. Example: `0`. |
| `apps` | integer | How many apps Steam records as installed here. |
| `mounted` | boolean | Whether the directory is reachable right now. An external drive that is unplugged is configured but not mounted. |
| `default` | boolean | Whether this is the Steam installation's own library. |
| `totalSize` | integer | Size of the drive as Steam last measured it, in bytes. |
<!-- whoctl:end status -->

## Columns

<!-- whoctl:begin columns -->
| Column | Shown |
| --- | --- |
| `PATH` | always |
| `LABEL` | always |
| `APPS` | always |
| `MOUNTED` | always |
| `INDEX` | `-o wide` |
| `DEFAULT` | `-o wide` |
<!-- whoctl:end columns -->

## Behaviour

**A library must exist before it can be added.** Steam drops an entry pointing
at a directory it cannot see on its next scan, so writing one would produce a
change that silently undoes itself.

**`mounted` is the interesting column.** An external drive that is unplugged is
still configured, and reporting it as absent would be wrong; reporting it as
present would be worse. The two facts are kept apart.

**`delete` forgets, it does not erase.** Removing a library from Steam's list
leaves every game in it exactly where it was. Deleting the Steam installation's
own library is refused outright.

**The default library is matched through symlinks.** `~/.steam/steam` is a link
to `~/.local/share/Steam` on most installations, and the two names are the same
library.
