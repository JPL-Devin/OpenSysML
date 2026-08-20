#!/usr/bin/env bash
# Build DeciSym's sysmlv2-validator (a thin CLI over the OMG SysML v2 Pilot
# Implementation) into build/pilot-validator/, for the advisory differential
# harness in cmd/pilot-diff. See docs/project/pilot-differential.md.
#
# Both the wrapper commit and the pilot release are pinned; the pilot tag and
# artifact version come from scripts/pilot-pin.sh, the pin the corpora use too.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

VALIDATOR_REPO="${VALIDATOR_REPO:-https://github.com/DeciSym/sysmlv2-validator.git}"
VALIDATOR_COMMIT="${VALIDATOR_COMMIT:-0d706e5ba1e9c56730cb8600ee43602906e12058}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/build/pilot-validator"

if [ -x "$target/validate-sysml" ] && [ -f "$target/target/sysmlv2-validator-1.0.0-SNAPSHOT.jar" ]; then
	echo "Pilot validator already built at $target"
	echo "Remove that directory to re-provision."
	exit 0
fi

for tool in git java mvn; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "error: $tool is required to build the pilot validator" >&2
		exit 1
	fi
done

java_major="$(java -version 2>&1 | sed -n '1s/.*version "\([0-9][0-9]*\).*/\1/p')"
if [ -z "$java_major" ] || [ "$java_major" -lt 21 ]; then
	echo "error: the pilot implementation requires Java 21+, found: $(java -version 2>&1 | head -1)" >&2
	exit 1
fi

mkdir -p "$(dirname "$target")"
rm -rf "$target"

echo "Cloning $VALIDATOR_REPO at $VALIDATOR_COMMIT ..."
git init --quiet "$target"
git -C "$target" remote add origin "$VALIDATOR_REPO"
git -C "$target" fetch --quiet --depth 1 origin "$VALIDATOR_COMMIT"
git -C "$target" checkout --quiet FETCH_HEAD

# Fail loudly rather than compare against an unexpected pilot release.
for pin in "sysml.release.tag:$PILOT_TAG" "sysml.artifact.version:$PILOT_ARTIFACT_VERSION"; do
	property="${pin%%:*}"
	want="${pin#*:}"
	got="$(sed -n "s|.*<${property}>\(.*\)</${property}>.*|\1|p" "$target/pom.xml" | head -1)"
	if [ "$got" != "$want" ]; then
		echo "error: $VALIDATOR_COMMIT builds against $property=$got, this repository pins $want" >&2
		echo "       re-pin VALIDATOR_COMMIT, or override PILOT_TAG/PILOT_ARTIFACT_VERSION deliberately" >&2
		exit 1
	fi
done

# The pilot is not on Maven Central: the setup-dependency profile downloads the
# jupyter-sysml-kernel release ZIP (jar + sysml.library) and installs the jar
# into ~/.m2, which `mvn package` then shades into the validator jar.
echo "Downloading the pilot $PILOT_TAG ($PILOT_ARTIFACT_VERSION) release and building the validator ..."
(cd "$target" && mvn -B -q -Psetup-dependency initialize && mvn -B -q package)

library="$target/target/sysml-download/sysml/sysml.library"
if [ ! -d "$library" ]; then
	echo "error: the pilot standard library is missing from $library" >&2
	exit 1
fi

echo "Built $target/validate-sysml (pilot $PILOT_TAG, $PILOT_ARTIFACT_VERSION)"
echo "Compare it against this implementation with:"
echo "  go run ./cmd/pilot-diff"
