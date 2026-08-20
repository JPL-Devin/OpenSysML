#!/usr/bin/env bash
# Download the OMG SysML v2 and KerML corpora the differential harness compares
# beyond the training corpus, into examples/pilot-corpora/.
#
# Like the training examples, these are not vendored: they belong to the OMG
# pilot implementation and are licensed there. The harness skips a root whose
# directory is absent, so this script is optional for building and testing.
#
# The tag is pinned in scripts/pilot-pin.sh, the same pin the training corpus and
# the reference validator use, so a comparison is always against one release.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

# Each entry is "<path in the pilot repository>:<directory under examples/pilot-corpora>".
CORPORA=(
	"sysml/src/examples:sysml-examples"
	"sysml/src/validation:sysml-validation"
	"kerml/src/examples:kerml-examples"
)

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
parent="$repo_root/examples/pilot-corpora"

declare -a wanted_paths=()
declare -a wanted_targets=()
for corpus in "${CORPORA[@]}"; do
	source_path="${corpus%%:*}"
	target="$parent/${corpus#*:}"
	if [ -d "$target" ]; then
		echo "Corpus already present at $target"
		echo "Remove that directory to re-download."
		continue
	fi
	wanted_paths+=("$source_path")
	wanted_targets+=("$target")
done

if [ "${#wanted_paths[@]}" -eq 0 ]; then
	exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Fetching ${wanted_paths[*]} from $PILOT_REPO at $PILOT_TAG ..."
git clone --quiet --filter=blob:none --sparse --depth 1 \
	--branch "$PILOT_TAG" "$PILOT_REPO" "$work/pilot" 2>&1 | grep -v "detached HEAD" || true
git -C "$work/pilot" sparse-checkout set "${wanted_paths[@]}"

mkdir -p "$parent"
for index in "${!wanted_paths[@]}"; do
	source_path="${wanted_paths[$index]}"
	target="${wanted_targets[$index]}"
	if [ ! -d "$work/pilot/$source_path" ]; then
		echo "error: $source_path is missing from $PILOT_REPO at $PILOT_TAG" >&2
		exit 1
	fi
	mv "$work/pilot/$source_path" "$target"
	count="$(find "$target" -name '*.sysml' -o -name '*.kerml' | wc -l | tr -d ' ')"
	echo "Downloaded $count model file(s) from $source_path to $target"
done

echo "Compare them against the pilot implementation with:"
echo "  go run ./cmd/pilot-diff"
