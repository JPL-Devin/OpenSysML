#!/usr/bin/env bash
# Fails when production (non-test) code imports grpc-go. It stays allowed in
# tests, cmd/conformance, generated code, and cmd/sysml-grpc/grpcserver.go.
set -euo pipefail
cd "$(dirname "$0")/.."

module=$(go list -m)
fail=0

# Packages whose non-test files may import grpc-go, and why:
#   api/proto        committed generated grpc bindings
#   cmd/conformance  dials with a real grpc-go client on purpose
allowed_pkgs="$module/api/proto $module/cmd/conformance"

while read -r pkg dir imports; do
	case " $allowed_pkgs " in
	*" $pkg "*) continue ;;
	esac
	case " $imports " in
	*" google.golang.org/grpc"*) ;;
	*) continue ;;
	esac
	if [[ "$pkg" = "$module/cmd/sysml-grpc" ]]; then
		# Only grpcserver.go, the -transport grpc path, may import grpc-go here.
		offenders=$(grep -l '"google\.golang\.org/grpc' "$dir"/*.go |
			grep -v '_test\.go$' | grep -v '/grpcserver\.go$' || true)
		if [[ -n "$offenders" ]]; then
			echo "grpc-go imported outside grpcserver.go in $pkg:"
			echo "$offenders"
			fail=1
		fi
		continue
	fi
	echo "forbidden: production package $pkg imports grpc-go"
	fail=1
done < <(go list -f '{{.ImportPath}} {{.Dir}} {{join .Imports " "}}' ./...)

if [[ "$fail" -ne 0 ]]; then
	echo "grpc-go must stay out of production code; see docs/reference/service-transports.md"
	exit 1
fi
echo "grpc-go imports confined to tests, conformance, generated code and -transport grpc"
