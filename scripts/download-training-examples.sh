#!/usr/bin/env bash
# Download the OMG SysML v2 training examples into examples/sysml-v2-training/.
#
# The corpus is not vendored: it belongs to the OMG pilot implementation and is
# licensed there. The tests that read it skip when it is absent, so this script
# is optional for building and running the suite — but `go test ./internal/core/model`
# only gates the corpus once it has been downloaded.
#
# The tag is pinned to the same pilot release the bundled standard library comes
# from, so the expected results in internal/core/model/testdata are reproducible.
set -euo pipefail

PILOT_TAG="${PILOT_TAG:-2026-05}"
PILOT_REPO="${PILOT_REPO:-https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation.git}"
TRAINING_PATH="sysml/src/training"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/examples/sysml-v2-training"

if [ -d "$target" ]; then
	echo "Training examples already present at $target"
	echo "Remove that directory to re-download."
	exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Fetching $TRAINING_PATH from $PILOT_REPO at $PILOT_TAG ..."
git clone --quiet --filter=blob:none --sparse --depth 1 \
	--branch "$PILOT_TAG" "$PILOT_REPO" "$work/pilot" 2>&1 | grep -v "detached HEAD" || true
git -C "$work/pilot" sparse-checkout set "$TRAINING_PATH"

if [ ! -d "$work/pilot/$TRAINING_PATH" ]; then
	echo "error: $TRAINING_PATH is missing from $PILOT_REPO at $PILOT_TAG" >&2
	exit 1
fi

mkdir -p "$(dirname "$target")"
mv "$work/pilot/$TRAINING_PATH" "$target"

count="$(find "$target" -name '*.sysml' | wc -l | tr -d ' ')"
echo "Downloaded $count .sysml training files to $target"
