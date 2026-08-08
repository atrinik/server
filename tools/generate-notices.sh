#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

# Literal backticks are Markdown, not shell expansion.
# shellcheck disable=SC2016
printf '%s\n' '# Third-party notices' '' \
  'This file is generated from `policy/dependencies.json` by' \
  '`tools/generate-notices.sh`. Do not edit it independently.' '' \
  '| Module | Version | SPDX license | Source |' \
  '| --- | --- | --- | --- |'
jq -r '.modules[] | "| `\(.name)` | `\(.version)` | `\(.license)` | \(.source) |"' policy/dependencies.json
printf '%s\n' '' \
  'This notice is a summary, not a replacement for upstream license texts or the' \
  'release SBOM. No GPL or AGPL dependency is approved.'
