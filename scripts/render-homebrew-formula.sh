#!/usr/bin/env bash
#
# Render the Homebrew formula for a published release.
#
# Usage:
#   scripts/render-homebrew-formula.sh <tag> [SHA256SUMS.txt] > systemica.rb
#
# <tag> is the release tag as it appears in the GitHub release URL (e.g. v0.3.0).
# If the checksum file is omitted it is downloaded from the release. The
# checksums are produced by the build-release job in .circleci/config.yml.
#
# The rendered formula is meant to be committed to a Homebrew tap repository;
# this repository does not host a tap. See packaging/homebrew/README.md.
set -euo pipefail

TAG="${1:-}"
SUMS="${2:-}"

if [ -z "$TAG" ]; then
  echo "usage: $0 <tag> [SHA256SUMS.txt]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/../packaging/homebrew/systemica.rb.template"

if [ ! -f "$TEMPLATE" ]; then
  echo "error: template not found at $TEMPLATE" >&2
  exit 1
fi

cleanup() { [ -n "${TMP_SUMS:-}" ] && rm -f "$TMP_SUMS"; return 0; }
trap cleanup EXIT

if [ -z "$SUMS" ]; then
  TMP_SUMS="$(mktemp)"
  URL="https://github.com/Open-MBEE/Systemica/releases/download/${TAG}/SHA256SUMS.txt"
  echo "Fetching ${URL}" >&2
  curl -fsSL "$URL" -o "$TMP_SUMS"
  SUMS="$TMP_SUMS"
fi

# VERSION is the tag without a leading "v", which is what Homebrew expects.
VERSION="${TAG#v}"

sum_for() {
  local archive="$1" sum
  sum="$(awk -v a="$archive" '$2 == a { print $1 }' "$SUMS")"
  if [ -z "$sum" ]; then
    echo "error: no checksum for $archive in $SUMS" >&2
    exit 1
  fi
  printf '%s' "$sum"
}

DARWIN_ARM64="$(sum_for systemica-darwin-arm64.tar.gz)"
DARWIN_AMD64="$(sum_for systemica-darwin-amd64.tar.gz)"
LINUX_ARM64="$(sum_for systemica-linux-arm64.tar.gz)"
LINUX_AMD64="$(sum_for systemica-linux-amd64.tar.gz)"

sed \
  -e "s|__TAG__|${TAG}|g" \
  -e "s|__VERSION__|${VERSION}|g" \
  -e "s|__SHA256_DARWIN_ARM64__|${DARWIN_ARM64}|g" \
  -e "s|__SHA256_DARWIN_AMD64__|${DARWIN_AMD64}|g" \
  -e "s|__SHA256_LINUX_ARM64__|${LINUX_ARM64}|g" \
  -e "s|__SHA256_LINUX_AMD64__|${LINUX_AMD64}|g" \
  "$TEMPLATE" |
  awk 'started || /^class /{ started = 1; print }' # drop the template's header comment
