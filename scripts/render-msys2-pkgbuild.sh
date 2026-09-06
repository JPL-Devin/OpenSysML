#!/usr/bin/env bash
#
# Render the MSYS2 PKGBUILD for a published release, ready to submit to
# msys2/MINGW-packages as mingw-w64-opensysml/PKGBUILD.
#
# Usage:
#   scripts/render-msys2-pkgbuild.sh <tag> [source.tar.gz] > PKGBUILD
#
# <tag> is the release tag (e.g. v0.3.0). The PKGBUILD builds from the GitHub
# source archive of that tag, which is not part of SHA256SUMS.txt, so the
# archive is downloaded (or read from the given file) and hashed here.
#
# The PKGBUILD is submitted to an external repository by a maintainer, by hand;
# nothing here writes anywhere. See packaging/msys2/README.md.
set -euo pipefail

TAG="${1:-}"
SRC="${2:-}"

if [[ -z "$TAG" ]]; then
  echo "usage: $0 <tag> [source.tar.gz]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${SCRIPT_DIR}/../packaging/msys2/PKGBUILD"

if [[ ! -f "$TEMPLATE" ]]; then
  echo "error: PKGBUILD template not found at $TEMPLATE" >&2
  exit 1
fi

OUT="$(mktemp)"
cleanup() { rm -f "$OUT" "${TMP_SRC:-}"; }
trap cleanup EXIT

# makepkg forbids hyphens in pkgver; only x.y.z[-(alpha|beta|rc)N] tags are accepted so that
# 0.6.0-rc1 -> 0.6.0rc1 stays unambiguous and orders before 0.6.0 for vercmp.
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(alpha|beta|rc)[0-9]*)?$ ]]; then
  echo "error: tag '$TAG' is not vX.Y.Z or vX.Y.Z-(alpha|beta|rc)N" >&2
  exit 2
fi
TAGVER="${TAG#v}"
VERSION="${TAGVER//-/}"

if [[ -z "$SRC" ]]; then
  TMP_SRC="$(mktemp)"
  URL="https://github.com/Open-MBEE/OpenSysML/archive/${TAG}/opensysml-${TAGVER}.tar.gz"
  echo "Fetching ${URL}" >&2
  curl -fsSL --proto '=https' --proto-redir '=https' "$URL" -o "$TMP_SRC"
  SRC="$TMP_SRC"
fi
SOURCE_SHA256="$(sha256sum "$SRC" | awk '{print $1}')"

sed \
  -e "s|__VERSION__|${VERSION}|g" \
  -e "s|__TAGVER__|${TAGVER}|g" \
  -e "s|__SHA256_SOURCE__|${SOURCE_SHA256}|g" \
  "$TEMPLATE" |
  awk 'started || /^_realname=/{ started = 1; print } !started && /^# Maintainer:/{ print; print "" }' > "$OUT" # keep the Maintainer line, drop the template note

if grep -q '__[A-Z0-9_]*__' "$OUT"; then
  echo "error: unsubstituted placeholders remain:" >&2
  grep -o '__[A-Z0-9_]*__' "$OUT" | sort -u >&2
  exit 1
fi

cat "$OUT"
