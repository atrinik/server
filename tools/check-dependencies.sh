#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

jq -e '
  .schema_version == 1 and
  (.modules | length > 0) and
  ([.modules[].license] - .allowed_spdx | length == 0) and
  ([.modules[].license] | all(test("^(AGPL|GPL)-") | not))
' policy/dependencies.json >/dev/null

actual=$(mktemp /tmp/atrinik-server-modules.XXXXXX)
expected=$(mktemp /tmp/atrinik-server-policy.XXXXXX)
licenses=$(mktemp /tmp/atrinik-server-licenses.XXXXXX)
trap 'rm -f -- "${actual}" "${expected}" "${licenses}"' EXIT

go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{end}}{{end}}' ./... \
  | sed '/^$/d' | sort -u >"${actual}"
jq -r '.modules[] | [.name, .version] | @tsv' policy/dependencies.json \
  | tr '\t' ' ' | sort -u >"${expected}"
diff -u "${expected}" "${actual}"

if grep -Eiq '(^|/)(atrinik/(classic|client|editor|renderer|content-toolkit)|python|cpython)(/|$)' "${actual}"; then
  echo "forbidden implementation dependency in effective package graph" >&2
  exit 1
fi

go-licenses report ./... >"${licenses}"
if grep -Eiq '(^|[,;[:space:]])(AGPL|GPL)-?[123](\.[0-9])?([+-]|$|[,;[:space:]])' "${licenses}"; then
  echo "reciprocal dependency license is not approved" >&2
  cat "${licenses}" >&2
  exit 1
fi

while IFS=, read -r _module _source license; do
  if ! jq -e --arg license "${license}" '.allowed_spdx | index($license) != null' policy/dependencies.json >/dev/null; then
    echo "dependency has an unapproved or unknown license: ${license}" >&2
    exit 1
  fi
done <"${licenses}"

generated=$(mktemp /tmp/atrinik-server-notices.XXXXXX)
trap 'rm -f -- "${actual}" "${expected}" "${licenses}" "${generated}"' EXIT
tools/generate-notices.sh >"${generated}"
diff -u THIRD_PARTY_NOTICES.md "${generated}"
