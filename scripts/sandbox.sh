#!/bin/sh
# Opens a shell on a throwaway machine with this provider ready to use, or runs
# one command there.
#
# The container harness is whoctl's — running whoctl and some providers on a
# throwaway machine is its job. What stays here is what is this provider's: a
# Steam installation for it to read.
#
# # What is prepared, and why it is mounted where it is
#
# The fixture goes to ~/.steam/steam, which is the first place the provider
# looks when nothing tells it otherwise. That is the point: on a workstation
# every command needs --root or WHOCTL_STEAM_ROOT, so detection — walking the
# four standard install locations in the order Steam itself prefers them — is
# the one part of this provider that nothing ever exercises. In here it is the
# part that runs.
#
# # The Web API half is not here
#
# Half this provider's kinds read api.steampowered.com, which needs somebody's
# real key and their real account. A sandbox cannot fake that and must not be
# handed the real one, so those kinds report a missing key in here, which is
# what they do on any machine without one. Set STEAM_API_KEY and STEAM_ID in
# your own shell if you want them: the harness passes nothing through, so it is
# a deliberate act.
#
# Usage:
#   scripts/sandbox.sh                        # a shell
#   scripts/sandbox.sh whoctl get steam/apps  # one command
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
sandbox="${WHOCTL_SANDBOX:-$root/../whoctl/scripts/sandbox.sh}"

if [ ! -x "$sandbox" ]; then
	echo "no sandbox to run in: check out github.com/whoctl/whoctl beside this" >&2
	echo "repository, or set WHOCTL_SANDBOX to its scripts/sandbox.sh." >&2
	exit 1
fi

cat >&2 <<'MSG'
steam sandbox — the fixture installation is at ~/.steam/steam, found by the
provider's own detection rather than by a flag. Inside:

  whoctl get steam/apps
  whoctl get steam/libraries

MSG

# Read-only: the local kinds do not write yet, and a fixture a sandbox could
# rewrite is a fixture that stops matching what the unit tests read.
PROVIDERS=steam \
MOUNTS="-v $root/testdata/steam:/root/.steam/steam:ro,z" \
	exec "$sandbox" "$@"
