#!/usr/bin/env bash
# Single source of the OMG SysML v2 Pilot Implementation pin, sourced by every
# script that fetches something from it: the training corpus, the additional OMG
# corpora, and the reference validator the differential harness compares against.
#
# Kept in one file so the release under comparison cannot drift between them.
PILOT_TAG="${PILOT_TAG:-2026-05}"
PILOT_REPO="${PILOT_REPO:-https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation.git}"
PILOT_ARTIFACT_VERSION="${PILOT_ARTIFACT_VERSION:-0.60.1}"

# pilot_fetch_subtrees copies model roots out of the pinned pilot repository in one
# sparse clone. Each argument is "<path in the pilot repository>:<destination>";
# only those paths are checked out and an existing destination is left alone, so
# the fetch stays as narrow and as pinned as the scripts it replaces.
pilot_fetch_subtrees() {
	local paths=() targets=() entry source_path target work index count
	for entry in "$@"; do
		source_path="${entry%%:*}"
		target="${entry#*:}"
		if [ -d "$target" ]; then
			echo "Already present at $target"
			echo "Remove that directory to re-download."
			continue
		fi
		paths+=("$source_path")
		targets+=("$target")
	done
	if [ "${#paths[@]}" -eq 0 ]; then
		return 0
	fi

	work="$(mktemp -d)"
	# shellcheck disable=SC2064 # expand $work now: the trap outlives this scope
	trap "rm -rf '$work'" EXIT

	echo "Fetching ${paths[*]} from $PILOT_REPO at $PILOT_TAG ..."
	git clone --quiet --filter=blob:none --sparse --depth 1 \
		--branch "$PILOT_TAG" "$PILOT_REPO" "$work/pilot" 2>&1 | grep -v "detached HEAD" || true
	git -C "$work/pilot" sparse-checkout set "${paths[@]}"

	for index in "${!paths[@]}"; do
		source_path="${paths[$index]}"
		target="${targets[$index]}"
		if [ ! -d "$work/pilot/$source_path" ]; then
			echo "error: $source_path is missing from $PILOT_REPO at $PILOT_TAG" >&2
			return 1
		fi
		mkdir -p "$(dirname "$target")"
		mv "$work/pilot/$source_path" "$target"
		count="$(find "$target" -type f \( -name '*.sysml' -o -name '*.kerml' \) | wc -l | tr -d ' ')"
		echo "Downloaded $count model file(s) from $source_path to $target"
	done
}
