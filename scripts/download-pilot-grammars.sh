#!/usr/bin/env bash
# Download the OMG Xtext grammars the grammar-coverage measurement reads, into
# build/pilot-grammars/.
#
# Like every other corpus we compare against, the grammars are not vendored:
# they belong to the OMG pilot implementation and are licensed there. The
# measurement is advisory and nothing in the build or the test suite reads the
# download, so this script is optional for building and testing.
#
# The tag is pinned in scripts/pilot-pin.sh, the same pin the training corpus,
# the additional OMG corpora and the reference validator use, so the grammars
# and the models are always from one release.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

# Each entry is "<path in the pilot repository>:<file name under build/pilot-grammars>".
GRAMMARS=(
	"org.omg.kerml.expressions.xtext/src/org/omg/kerml/expressions/xtext/KerMLExpressions.xtext:KerMLExpressions.xtext"
	"org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext:KerML.xtext"
	"org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext:SysML.xtext"
)

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/build/pilot-grammars"

declare -a wanted_paths=()
declare -a wanted_names=()
for grammar in "${GRAMMARS[@]}"; do
	source_path="${grammar%%:*}"
	name="${grammar#*:}"
	if [ -f "$target/$name" ]; then
		echo "Grammar already present at $target/$name"
		echo "Remove that file to re-download."
		continue
	fi
	wanted_paths+=("$source_path")
	wanted_names+=("$name")
done

if [ "${#wanted_paths[@]}" -eq 0 ]; then
	exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Fetching ${wanted_names[*]} from $PILOT_REPO at $PILOT_TAG ..."
git clone --quiet --filter=blob:none --sparse --depth 1 \
	--branch "$PILOT_TAG" "$PILOT_REPO" "$work/pilot" 2>&1 | grep -v "detached HEAD" || true
git -C "$work/pilot" sparse-checkout set "${wanted_paths[@]}"

mkdir -p "$target"
for index in "${!wanted_paths[@]}"; do
	source_path="${wanted_paths[$index]}"
	name="${wanted_names[$index]}"
	if [ ! -f "$work/pilot/$source_path" ]; then
		echo "error: $source_path is missing from $PILOT_REPO at $PILOT_TAG" >&2
		exit 1
	fi
	mv "$work/pilot/$source_path" "$target/$name"
	lines="$(wc -l <"$target/$name" | tr -d ' ')"
	echo "Downloaded $name ($lines lines) from $source_path"
done

# Recorded so the report can name the release it measured.
printf '%s\n' "$PILOT_TAG" >"$target/PILOT_TAG"

echo "Measure our production coverage against them with:"
echo "  go run ./cmd/grammar-coverage"
