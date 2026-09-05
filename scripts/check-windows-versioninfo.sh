#!/usr/bin/env bash
# Checks that a Windows binary's VERSIONINFO resource carries the version it is
# released as. `make windows-versioninfo-check EXE=... VERSION=...` runs it with
# GO_WINRES set; run standalone it needs GO_WINRES or a go-winres on PATH.
#
#   scripts/check-windows-versioninfo.sh dist/sysml-windows-amd64.exe v0.5.0
#
# Fails unless ProductVersion and FileVersion both equal the version and
# ProductName is "OpenSysML", which SignPath's file metadata restrictions
# enforce at signing time.
set -euo pipefail

exe="${1:?usage: $0 <file.exe> <version>}"
version="${2:?usage: $0 <file.exe> <version>}"
GO_WINRES="${GO_WINRES:-go-winres}"

if ! command -v jq >/dev/null 2>&1; then
  echo "Error: jq is required" >&2
  exit 2
fi
if [ ! -f "$exe" ]; then
  echo "Error: $exe does not exist" >&2
  exit 2
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# GO_WINRES may be a command line such as "go run <module>@<version>".
# shellcheck disable=SC2086
$GO_WINRES extract --dir "$tmp" "$exe" >/dev/null

info="$tmp/winres.json"
if [ ! -f "$info" ]; then
  echo "Error: $exe carries no resources at all" >&2
  exit 1
fi

# The one VERSIONINFO block, whatever language it was written under.
strings_json="$(jq -c '[.RT_VERSION[]?[]?.info[]?] | first // empty' "$info")"
if [ -z "$strings_json" ]; then
  echo "Error: $exe carries no VERSIONINFO resource" >&2
  exit 1
fi

field() { printf '%s' "$strings_json" | jq -r --arg k "$1" '.[$k] // ""'; }

status=0
expect() {
  local key="$1" want="$2" got
  got="$(field "$key")"
  if [ "$got" = "$want" ]; then
    echo "ok: $exe $key = $got"
  else
    echo "Error: $exe $key is '$got', expected '$want'" >&2
    status=1
  fi
}

expect ProductName "OpenSysML"
expect ProductVersion "$version"
expect FileVersion "$version"
for key in CompanyName FileDescription LegalCopyright OriginalFilename; do
  if [ -z "$(field "$key")" ]; then
    echo "Error: $exe has an empty $key" >&2
    status=1
  fi
done
exit $status
