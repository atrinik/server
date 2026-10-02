#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

fixture=$(mktemp -d /tmp/atrinik-server-dependency-test.XXXXXX)
trap 'rm -rf -- "${fixture}"' EXIT

mkdir -p "${fixture}/bin" "${fixture}/policy" "${fixture}/tools"
cp policy/dependencies.json "${fixture}/policy/"
cp tools/check-dependencies.sh tools/generate-notices.sh "${fixture}/tools/"

jq '.modules += [{
  "name": "example.com/windows-only",
  "license": "MIT",
  "source": "https://example.com/windows-only"
}]' "${fixture}/policy/dependencies.json" >"${fixture}/policy/dependencies.next.json"
mv "${fixture}/policy/dependencies.next.json" "${fixture}/policy/dependencies.json"
jq -r '.modules[].name | select(. != "example.com/windows-only")' \
  "${fixture}/policy/dependencies.json" >"${fixture}/modules.txt"
printf '%s\n' 'example.com/windows-only' >"${fixture}/windows-modules.txt"

cat >"${fixture}/bin/git" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == "rev-parse --show-toplevel" ]]; then
  pwd
  exit 0
fi
echo "unexpected git invocation: $*" >&2
exit 1
EOF

cat >"${fixture}/bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" != "list" ]]; then
  echo "unexpected go invocation: $*" >&2
  exit 1
fi
if [[ "$*" == *'.Version'* ]]; then
  echo "dependency policy requested a module version" >&2
  exit 1
fi
cat modules.txt
if [[ "${GOOS:-}" == "windows" ]]; then
  cat windows-modules.txt
fi
EOF

cat >"${fixture}/bin/go-licenses" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" != "report" ]]; then
  echo "unexpected go-licenses invocation: $*" >&2
  exit 1
fi
jq -r --arg override "${MOCK_LICENSE_OVERRIDE:-}" \
  '.modules[] | [.name, .source, (if $override == "" then .license else $override end)] | @csv' \
  policy/dependencies.json \
  | tr -d '"'
EOF

chmod +x "${fixture}/bin/git" "${fixture}/bin/go" "${fixture}/bin/go-licenses" \
  "${fixture}/tools/check-dependencies.sh" "${fixture}/tools/generate-notices.sh"

fixture_path="${fixture}/bin:${PATH}"
(
  cd "${fixture}"
  PATH="${fixture_path}" tools/generate-notices.sh >THIRD_PARTY_NOTICES.md
  PATH="${fixture_path}" tools/check-dependencies.sh
  grep -Fq '| `github.com/atrinik/protocol` | `MIT` |' THIRD_PARTY_NOTICES.md
  grep -Fq '| `example.com/windows-only` | `MIT` |' THIRD_PARTY_NOTICES.md

  if output=$(PATH="${fixture_path}" MOCK_LICENSE_OVERRIDE=GPL-3.0-only \
      tools/check-dependencies.sh 2>&1); then
    echo "reciprocal effective dependency license unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq 'reciprocal dependency license is not approved' <<<"${output}"

  jq '.modules += [{
    "name": "github.com/atrinik/classic/bridge",
    "license": "MIT",
    "source": "https://github.com/atrinik/classic"
  }]' policy/dependencies.json >policy/dependencies.next.json
  mv policy/dependencies.next.json policy/dependencies.json
  printf '%s\n' 'github.com/atrinik/classic/bridge' >>modules.txt
  PATH="${fixture_path}" tools/generate-notices.sh >THIRD_PARTY_NOTICES.md

  if output=$(PATH="${fixture_path}" tools/check-dependencies.sh 2>&1); then
    echo "forbidden implementation dependency unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq 'forbidden implementation dependency in effective package graph' \
    <<<"${output}"
)
