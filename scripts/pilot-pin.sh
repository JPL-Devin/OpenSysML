#!/usr/bin/env bash
# Single source of the OMG SysML v2 Pilot Implementation pin, sourced by every
# script that fetches something from it: the training corpus, the additional OMG
# corpora, and the reference validator the differential harness compares against.
#
# Kept in one file so the release under comparison cannot drift between them.
PILOT_TAG="${PILOT_TAG:-2026-07}"
PILOT_REPO="${PILOT_REPO:-https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation.git}"
PILOT_ARTIFACT_VERSION="${PILOT_ARTIFACT_VERSION:-0.61.0}"

# pilot_fetch_subtrees copies model roots out of the pinned pilot repository in one
# sparse clone. Each argument is "<path in the pilot repository>:<destination>";
# only those paths are checked out. Each destination records the tag it was
# fetched at in a .pilot-pin stamp: a destination stamped with the current tag is
# left alone, one stamped with another tag is re-downloaded, and one without a
# stamp is left alone with a warning, since its pin cannot be verified.
pilot_fetch_subtrees() {
	local paths=() targets=() entry source_path target work index count stamp
	for entry in "$@"; do
		source_path="${entry%%:*}"
		target="${entry#*:}"
		if [[ -d "$target" ]]; then
			stamp="$target/.pilot-pin"
			if [[ -f "$stamp" ]] && [[ "$(cat "$stamp")" == "$PILOT_TAG" ]]; then
				echo "Already present at $target (pin $PILOT_TAG)"
				echo "Remove that directory to re-download."
				continue
			fi
			if [[ -f "$stamp" ]]; then
				echo "Stale pin at $target: fetched at $(cat "$stamp"), pin is now $PILOT_TAG; re-downloading."
				rm -rf "$target"
			else
				echo "warning: $target exists but records no pin; leaving it alone." >&2
				echo "warning: remove that directory to re-download at $PILOT_TAG." >&2
				continue
			fi
		fi
		paths+=("$source_path")
		targets+=("$target")
	done
	if [[ "${#paths[@]}" -eq 0 ]]; then
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
		if [[ ! -d "$work/pilot/$source_path" ]]; then
			echo "error: $source_path is missing from $PILOT_REPO at $PILOT_TAG" >&2
			return 1
		fi
		mkdir -p "$(dirname "$target")"
		mv "$work/pilot/$source_path" "$target"
		printf '%s\n' "$PILOT_TAG" >"$target/.pilot-pin"
		count="$(find "$target" -type f \( -name '*.sysml' -o -name '*.kerml' \) | wc -l | tr -d ' ')"
		echo "Downloaded $count model file(s) from $source_path to $target"
	done
}
