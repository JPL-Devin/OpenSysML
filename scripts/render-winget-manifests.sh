#!/usr/bin/env bash
#
# Render the winget manifest set for a published release into the directory
# layout microsoft/winget-pkgs expects:
#   <outdir>/manifests/o/OpenMBEE/OpenSysML/<version>/OpenMBEE.OpenSysML{,.installer,.locale.en-US}.yaml
#
# Usage:
#   scripts/render-winget-manifests.sh <tag> <outdir> [SHA256SUMS.txt]
#
# <tag> is the release tag as it appears in the GitHub release URL (e.g. v0.3.0).
# If the checksum file is omitted it is downloaded from the release. The
# checksums are produced by the build-release job in .circleci/config.yml.
#
# The manifests are submitted to microsoft/winget-pkgs by a maintainer, by hand;
# nothing here writes anywhere but <outdir>. See packaging/winget/README.md.
set -euo pipefail

TAG="${1:-}"
OUTDIR="${2:-}"
SUMS="${3:-}"

if [[ -z "$TAG" || -z "$OUTDIR" ]]; then
  echo "usage: $0 <tag> <outdir> [SHA256SUMS.txt]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_DIR="${SCRIPT_DIR}/../packaging/winget/OpenMBEE.OpenSysML"
PACKAGE_ID="OpenMBEE.OpenSysML"

if [[ ! -d "$TEMPLATE_DIR" ]]; then
  echo "error: manifest templates not found at $TEMPLATE_DIR" >&2
  exit 1
fi

cleanup() { rm -f "${TMP_SUMS:-}"; }
trap cleanup EXIT

if [[ -z "$SUMS" ]]; then
  TMP_SUMS="$(mktemp)"
  URL="https://github.com/Open-MBEE/OpenSysML/releases/download/${TAG}/SHA256SUMS.txt"
  echo "Fetching ${URL}" >&2
  curl -fsSL --proto '=https' --proto-redir '=https' "$URL" -o "$TMP_SUMS"
  SUMS="$TMP_SUMS"
fi

sum_for() {
  local archive="$1" sum
  sum="$(awk -v a="$archive" '$2 == a { print $1 }' "$SUMS")"
  if [[ -z "$sum" ]]; then
    echo "error: no checksum for $archive in $SUMS" >&2
    exit 1
  fi
  printf '%s' "$sum"
}

# winget PackageVersion carries no "v".
VERSION="${TAG#v}"
WINDOWS_AMD64="$(sum_for opensysml-windows-amd64.zip)"

# manifests/<first letter of publisher, lower-cased>/<Publisher>/<Package>/<version>/
DEST="${OUTDIR}/manifests/o/OpenMBEE/OpenSysML/${VERSION}"
mkdir -p "$DEST"

for template in "$TEMPLATE_DIR"/"$PACKAGE_ID"*.yaml; do
  out="${DEST}/$(basename "$template")"
  sed \
    -e "s|__TAG__|${TAG}|g" \
    -e "s|__VERSION__|${VERSION}|g" \
    -e "s|__SHA256_WINDOWS_AMD64__|${WINDOWS_AMD64}|g" \
    "$template" |
    grep -v '^# Template: ' > "$out" # drop the maintainer-facing template note
  if grep -q '__[A-Z0-9_]*__' "$out"; then
    echo "error: unsubstituted placeholders remain in $out:" >&2
    grep -o '__[A-Z0-9_]*__' "$out" | sort -u >&2
    exit 1
  fi
done

ls "$DEST"/*.yaml
