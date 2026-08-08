#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

test "$(go version | awk '{print $3}')" = go1.26.5
for command in go staticcheck govulncheck go-licenses jq syft; do
  command -v "${command}" >/dev/null || { echo "missing required tool: ${command}" >&2; exit 1; }
done

unformatted=$(gofmt -l .)
test -z "${unformatted}" || { echo "unformatted Go files:" >&2; echo "${unformatted}" >&2; exit 1; }
go mod verify
go vet ./...
staticcheck ./...
go test ./...
go test -race ./...
go test -run=^$ -fuzz=FuzzJSONEventNeverEmitsSecret -fuzztime=1x ./internal/observability
go test -run=^$ -bench=. -benchtime=1x ./internal/kernel
govulncheck ./...
tools/check-foundations.sh
tools/check-dependencies.sh

temporary=$(mktemp -d /tmp/atrinik-server-validation.XXXXXX)
trap 'rm -rf -- "${temporary}"' EXIT
rmdir "${temporary}"
tools/package-release.sh "${temporary}" 0.1.0-test.1
test -s "${temporary}/SHA256SUMS"

second=$(mktemp -d /tmp/atrinik-server-reproducibility.XXXXXX)
rmdir "${second}"
tools/package-release.sh "${second}" 0.1.0-test.1
for artifact in atrinik-server-0.1.0-test.1-linux-amd64.tar.gz atrinik-server-0.1.0-test.1-windows-amd64.zip atrinik-server-0.1.0-test.1-source.tar.gz; do
  cmp "${temporary}/${artifact}" "${second}/${artifact}"
done
rm -rf -- "${second}"

git diff --check
