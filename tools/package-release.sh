#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

output=${1:-dist}
version=${2:-$(git describe --tags --always --dirty)}
if [[ -n $(git status --porcelain) ]]; then
  echo "release packaging requires a clean tracked and untracked worktree" >&2
  exit 1
fi
if [[ -e ${output} ]]; then
  echo "release output already exists: ${output}" >&2
  exit 1
fi
if [[ ! ${version} =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
  echo "release version must be semantic and omit the v prefix: ${version}" >&2
  exit 1
fi

revision=$(git rev-parse HEAD)
temporary=$(mktemp -d /tmp/atrinik-server-release.XXXXXX)
trap 'rm -rf -- "${temporary}"' EXIT
install -d "${output}" "${temporary}/linux" "${temporary}/windows"
output=$(realpath "${output}")

common_ldflags=(-buildid= -X "github.com/atrinik/server/internal/buildinfo.version=${version}" -X "github.com/atrinik/server/internal/buildinfo.revision=${revision}")
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "${common_ldflags[*]}" -o "${temporary}/linux/atrinik-server" ./cmd/atrinik-server
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "${common_ldflags[*]}" -o "${temporary}/windows/atrinik-server.exe" ./cmd/atrinik-server

for platform in linux windows; do
  cp LICENSE PROVENANCE.md THIRD_PARTY_NOTICES.md "${temporary}/${platform}/"
done
TZ=UTC touch -t 198001010000 "${temporary}"/windows/*

tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
  -C "${temporary}/linux" -cf - . | gzip -n >"${output}/atrinik-server-${version}-linux-amd64.tar.gz"
(
  cd "${temporary}/windows"
  zip -X -q -r "${output}/atrinik-server-${version}-windows-amd64.zip" .
)
git archive --format=tar --prefix="atrinik-server-${version}/" HEAD \
  | gzip -n >"${output}/atrinik-server-${version}-source.tar.gz"

SYFT_CHECK_FOR_APP_UPDATE=false syft dir:"${temporary}" \
  --source-name atrinik-server --source-version "${version}" \
  --output "cyclonedx-json=${output}/sbom.cdx.json"
cp policy/dependencies.json "${output}/dependency-policy.json"
jq -n \
  --arg version "${version}" --arg revision "${revision}" \
  --arg go "$(go version)" --arg syft "$(syft version -o json | jq -r .version)" \
  '{schema_version: 1, version: $version, revision: $revision,
    targets: ["linux/amd64", "windows/amd64"],
    cgo_enabled: false, trimpath: true,
    protocol_contract: "game-protocol-1", content_format_range: ">=1.0.0 <2.0.0",
    tools: {go: $go, syft: $syft}}' >"${output}/provenance.json"
cp LICENSE PROVENANCE.md THIRD_PARTY_NOTICES.md "${output}/"

checksums=$(mktemp /tmp/atrinik-server-checksums.XXXXXX)
(
  cd "${output}"
  find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum
) >"${checksums}"
mv "${checksums}" "${output}/SHA256SUMS"

tar -xOf "${output}/atrinik-server-${version}-linux-amd64.tar.gz" ./atrinik-server >"${temporary}/linux-check"
chmod +x "${temporary}/linux-check"
"${temporary}/linux-check" version | jq -e --arg version "${version}" --arg revision "${revision}" '.version == $version and .revision == $revision' >/dev/null
unzip -tqq "${output}/atrinik-server-${version}-windows-amd64.zip"
