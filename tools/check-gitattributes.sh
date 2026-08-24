#!/usr/bin/env bash
set -euo pipefail

repository=$(git rev-parse --show-toplevel)
cd "${repository}"

command -v git-lfs >/dev/null || {
  echo "Git LFS is required for the attribute regression check" >&2
  exit 1
}

before=$(git status --porcelain=v1 --untracked-files=all)
if [[ -n ${before} ]]; then
  echo "attribute regression check requires a clean checkout before Git LFS initialization" >&2
  printf '%s\n' "${before}" >&2
  exit 1
fi

git lfs install --local --force >/dev/null

while IFS= read -r -d '' path; do
  case "${path}" in
    *.flac|*.gif|*.ico|*.it|*.jpeg|*.jpg|*.mid|*.midi|*.mp3|*.ogg|*.opus|*.otf|*.s3m|*.ttf|*.wav|*.webp|*.woff|*.woff2|*.xm|*.zip|*.png)
      ;;
    *)
      attribute=$(git check-attr filter -- "${path}")
      expected="${path}: filter: unspecified"
      if [[ ${attribute} != "${expected}" ]]; then
        echo "source/config path unexpectedly uses Git LFS: ${attribute}" >&2
        exit 1
      fi
      ;;
  esac
done < <(git ls-files -z)

test "$(git check-attr filter -- go.mod)" = 'go.mod: filter: unspecified'

after=$(git status --porcelain=v1 --untracked-files=all)
if [[ -n ${after} ]]; then
  echo "Git LFS initialization changed the clean checkout" >&2
  printf '%s\n' "${after}" >&2
  exit 1
fi
