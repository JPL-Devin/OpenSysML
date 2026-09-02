#!/usr/bin/env bash
# Download the two inputs cmd/ontology-modules partitions the SysML v2 OWL
# ontology from, into build/ontology-sources/:
#
#   sysmlv2-rdf-ontology/   the Open-MBEE OWL rendering of the metamodel
#                           (sysml2/owl/www.omg.org/spec/SysML.owl), the file
#                           internal/core/rdf/ontology/gen reads too
#   omg-xmi/KerML.xmi       the normative OMG XMI of KerML and SysML, whose
#   omg-xmi/SysML.xmi       package structure decides which module a term goes in
#
# Neither is vendored: the OWL is Open-MBEE's and the XMI is OMG's. Both are
# pinned — the ontology by commit, the XMI by SHA-256 — so a regeneration is
# reproducible, and the generated modules record the pins in their headers.
set -euo pipefail

ONTOLOGY_REPO="${ONTOLOGY_REPO:-https://github.com/Open-MBEE/sysmlv2-rdf-ontology.git}"
ONTOLOGY_COMMIT="${ONTOLOGY_COMMIT:-a1fda2623d4a0a7b7ace855e12e799dbd5cbaa82}"

# The XMI release whose package structure matches the ontology's metamodel
# version: the 172 classes SysML.owl 202407 declares are exactly the classes of
# the 20240201 KerML and SysML XMI. A later XMI renames and adds metaclasses.
XMI_RELEASE="${XMI_RELEASE:-20240201}"
KERML_XMI_SHA256="1fb54dc5d734d941cc734fb90fee2895b245e73939fe8abf050707a5748b7e79"
SYSML_XMI_SHA256="94536423d70b621c16f1386d4f82f5f09517be285448f918411cacbbe307cbcc"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/build/ontology-sources"
stamp="$target/.pin"
pin="$ONTOLOGY_COMMIT $XMI_RELEASE $KERML_XMI_SHA256 $SYSML_XMI_SHA256"

if [[ -f "$stamp" ]] && [[ "$(cat "$stamp")" == "$pin" ]]; then
	echo "Ontology sources already present at $target (ontology $ONTOLOGY_COMMIT, XMI $XMI_RELEASE)"
	echo "Remove that directory to re-download."
	exit 0
fi
if [[ -f "$stamp" ]]; then
	echo "Stale pin at $target: fetched from $(cat "$stamp"); re-downloading."
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
staged="$work/ontology-sources"
mkdir -p "$staged/omg-xmi"

echo "Fetching $ONTOLOGY_REPO at $ONTOLOGY_COMMIT ..."
git init --quiet "$staged/sysmlv2-rdf-ontology"
git -C "$staged/sysmlv2-rdf-ontology" remote add origin "$ONTOLOGY_REPO"
git -C "$staged/sysmlv2-rdf-ontology" fetch --quiet --depth 1 origin "$ONTOLOGY_COMMIT"
git -C "$staged/sysmlv2-rdf-ontology" -c advice.detachedHead=false checkout --quiet FETCH_HEAD
head="$(git -C "$staged/sysmlv2-rdf-ontology" rev-parse 'HEAD^{commit}')"
if [[ "$head" != "$ONTOLOGY_COMMIT" ]]; then
	echo "error: checked out $head, expected $ONTOLOGY_COMMIT" >&2
	exit 1
fi

fetch_xmi() {
	local name="$1" sha="$2" url="https://www.omg.org/spec/$1/$XMI_RELEASE/$1.xmi" out="$staged/omg-xmi/$1.xmi" got
	echo "Fetching $url ..."
	curl --fail --silent --show-error --location --output "$out" "$url"
	got="$(sha256sum "$out" | cut -d' ' -f1)"
	if [[ "$got" != "$sha" ]]; then
		echo "error: $url has SHA-256 $got, this script pins $sha" >&2
		echo "       OMG republished the file: compare before re-pinning" >&2
		return 1
	fi
	echo "Downloaded $name.xmi ($(wc -c <"$out" | tr -d ' ') bytes)"
}
fetch_xmi KerML "$KERML_XMI_SHA256"
fetch_xmi SysML "$SYSML_XMI_SHA256"

printf '%s\n' "$pin" >"$staged/.pin"

mkdir -p "$(dirname "$target")"
rm -rf "$target.new"
mv "$staged" "$target.new"
rm -rf "$target"
mv "$target.new" "$target"

echo "Regenerate the modules with:"
echo "  go run ./cmd/ontology-modules"
