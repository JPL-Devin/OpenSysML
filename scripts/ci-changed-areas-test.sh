#!/usr/bin/env bash
# Checks scripts/ci-changed-areas.sh against representative pull requests, in a
# throwaway repository so the cases are the only history.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/ci-changed-areas.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

git -C "$work" init -q
git -C "$work" config user.email ci@example.com
git -C "$work" config user.name CI
mkdir -p "$work/scripts"
cp "$script" "$work/scripts/"
git -C "$work" add scripts
git -C "$work" -c commit.gpgsign=false commit -qm base
base=$(git -C "$work" rev-parse HEAD)

failures=0

# case <name> <expected areas> <files...>
case_() {
  local name=$1 expected=$2
  shift 2
  git -C "$work" -c advice.detachedHead=false checkout -q "$base"
  git -C "$work" checkout -q -B "test-$name"
  for file in "$@"; do
    mkdir -p "$work/$(dirname "$file")"
    echo change >>"$work/$file"
    git -C "$work" add "$file"
  done
  git -C "$work" -c commit.gpgsign=false commit -qm "$name"

  local actual
  actual=$(cd "$work" && bash scripts/ci-changed-areas.sh "$base" HEAD 2>/dev/null |
    grep '=true$' | cut -d= -f1 | sort | paste -sd, -)
  if [ "$actual" != "$expected" ]; then
    echo "FAIL $name: expected [$expected], got [$actual]" >&2
    failures=$((failures + 1))
  else
    echo "ok   $name: $actual"
  fi
}

case_ docs-only docs docs/guide/index.md
case_ changelog-only docs CHANGELOG.md
case_ java-only java clients/java/opensysml-client/pom.xml
case_ node-only node clients/node/package.json
case_ python-only python python/opensysml/connection.py
case_ vscode-only vscode editors/vscode/package.json
# Any markdown counts as documentation: the site links out to repository files.
case_ two-client-readmes docs,java,node clients/java/README.md clients/node/README.md
case_ two-clients java,node clients/java/pom.xml clients/node/tsconfig.json
case_ go-source docs,go,java,node,python,vscode internal/core/parser/parser.go
case_ proto docs,go,java,node,python,vscode api/proto/sysml.proto
case_ conformance docs,go,java,node,python,vscode conformance/scenarios/01-server-info.json
case_ workflow docs,go,java,node,python,vscode .github/workflows/pr.yml
case_ unclaimed docs,go,java,node,python,vscode some-new-top-level/thing.txt

if [ "$failures" -ne 0 ]; then
  echo "$failures case(s) failed" >&2
  exit 1
fi
echo "all cases passed"
