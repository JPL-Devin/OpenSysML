#!/usr/bin/env bash
# Download the OMG pilot implementation's own Xpect test suites into
# build/pilot-xpect-corpus/. Each suite ships its .xt files, the /src dependency
# models they import and its own copy of the standard library, so the harness
# can load exactly the resource set an XPECT_SETUP block declares.
#
# Like the other corpora these are not vendored: they belong to the OMG pilot
# implementation and are licensed there. cmd/pilot-xpect skips a suite whose
# directory is absent, so this script is optional for building and testing.
#
# They live under build/ rather than examples/ deliberately: the .kerml and
# .sysml models the suites ship are inputs to this harness, not models this
# repository ships, and everything that walks examples/ would otherwise adopt
# them.
#
# The tag is pinned in scripts/pilot-pin.sh, the same pin the corpora and the
# reference validators use, so the declared expectations and the observed
# behaviour always come from one release.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

# Each entry is "<plugin directory in the pilot repository>:<directory under build/pilot-xpect-corpus>".
SUITES=(
	"org.omg.kerml.xpect.tests:kerml"
	"org.omg.sysml.xpect.tests:sysml"
)

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
parent="$repo_root/build/pilot-xpect-corpus"

declare -a wanted_paths=()
declare -a wanted_targets=()
for suite in "${SUITES[@]}"; do
	source_path="${suite%%:*}"
	target="$parent/${suite#*:}"
	if [[ -d "$target" ]]; then
		echo "Suite already present at $target"
		echo "Remove that directory to re-download."
		continue
	fi
	wanted_paths+=("$source_path")
	wanted_targets+=("$target")
done

if [[ "${#wanted_paths[@]}" -eq 0 ]]; then
	exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Fetching ${wanted_paths[*]} from $PILOT_REPO at $PILOT_TAG ..."
git -c advice.detachedHead=false clone --quiet --filter=blob:none --sparse --depth 1 \
	--branch "$PILOT_TAG" "$PILOT_REPO" "$work/pilot"
git -C "$work/pilot" sparse-checkout set "${wanted_paths[@]}"

mkdir -p "$parent"
total=0
for index in "${!wanted_paths[@]}"; do
	source_path="${wanted_paths[$index]}"
	target="${wanted_targets[$index]}"
	if [[ ! -d "$work/pilot/$source_path" ]]; then
		echo "error: $source_path is missing from $PILOT_REPO at $PILOT_TAG" >&2
		exit 1
	fi
	mv "$work/pilot/$source_path" "$target"
	count="$(find "$target" -name '*.xt' | wc -l | tr -d ' ')"
	total=$((total + count))
	echo "Downloaded $count .xt file(s) from $source_path to $target"
done

echo "Total $total .xt file(s) (the pin declares 303 KerML + 126 SysML = 429)."
echo "Compare our behaviour against their declared expectations with:"
echo "  go run ./cmd/pilot-xpect"
