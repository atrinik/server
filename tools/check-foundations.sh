#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

jq -e '
  .schema_version == 1 and
  .source.repository == "atrinik/content" and
  .source.revision == "60a3a53c4e3e64293cdba21c91caf0c7a618764d" and
  .source.sha256 == "4536b668a88e22c76b9a7a48878f5b51872120143679782f5137f4ae9f1ae2ce" and
  .inventory.python_rows == 177 and
  .inventory.production_runtime_rows == 117 and
  .inventory.test_and_mock_rows == 13 and
  .inventory.offline_toolkit_rows == 47 and
  .inventory.unowned_rows == 0 and
  .inventory.python_compatibility_host == false and
  ([.replacement_classes[]] | add) == 177 and
  .runtime_python == "forbidden"
' migration/classic-behavior.json >/dev/null

required=(CONTRIBUTING.md PROVENANCE.md SECURITY.md THIRD_PARTY_NOTICES.md)
for document in "${required[@]}"; do
  test -s "${document}"
done
tools/check-provenance-identity-reference.sh

if grep -RhE '^[[:space:]]*uses:' .github/workflows \
  | grep -Ev '@[0-9a-f]{40}([[:space:]]|$)' >/dev/null; then
  echo "workflow action is not pinned to an immutable commit" >&2
  exit 1
fi

if find . -path ./.git -prune -o -type f \( -name '*_generated.go' -o -name '*.pb.go' \) -print -quit | grep -q .; then
  echo "generated Go was added without a registered generator/drift contract" >&2
  exit 1
fi

go list -deps ./... | grep -Eiq '(^|/)(atrinik/(classic|client|editor|renderer|content-toolkit)|python|cpython)(/|$)' && {
  echo "forbidden classic, sibling, or Python dependency" >&2
  exit 1
}
exit 0
