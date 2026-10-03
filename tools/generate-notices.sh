#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

modules=$(mktemp /tmp/atrinik-server-notice-modules.XXXXXX)
trap 'rm -f -- "${modules}"' EXIT

{
  go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}' ./...
  GOOS=windows GOARCH=amd64 go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}' ./...
} | sed '/^$/d' | sort -u >"${modules}"

# Literal backticks are Markdown, not shell expansion.
# shellcheck disable=SC2016
printf '%s\n' '# Third-party notices' '' \
  'This file is generated from the effective Go dependency graph and' \
  '`policy/dependencies.json` license metadata by' \
  '`tools/generate-notices.sh`. Do not edit it independently.' '' \
  '| Module | SPDX license | Source |' \
  '| --- | --- | --- |'
while read -r name; do
  row=$(jq -r --arg name "${name}" \
    '.modules[] | select(.name == $name) | "| `\(.name)` | `\(.license)` | \(.source) |"' \
    policy/dependencies.json)
  if [[ -z "${row}" ]]; then
    echo "dependency lacks notice metadata: ${name}" >&2
    exit 1
  fi
  printf '%s\n' "${row}"
done <"${modules}"
printf '%s\n' '' \
  'This notice is a summary, not a replacement for upstream license texts or the' \
  'release SBOM. No GPL or AGPL dependency is approved.'
