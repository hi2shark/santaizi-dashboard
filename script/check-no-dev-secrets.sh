#!/usr/bin/env bash
# Fail if development credentials are tracked or if tracked files contain assigned
# Cloudflare Access / API token values. Does not print secret values.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: not a git repository" >&2
  exit 1
fi

fail=0

tracked_env="$(git ls-files | grep -E '(^|/)\.env($|\.)' | grep -vE '(^|/)\.env\.example$' || true)"
if [[ -n "$tracked_env" ]]; then
  echo "error: tracked env files are not allowed:" >&2
  printf '%s\n' "$tracked_env" >&2
  fail=1
fi

scan() {
  local pattern="$1"
  local matches
  matches="$(git grep -nI -E "$pattern" -- ':!.env.example' ':!script/check-no-dev-secrets.sh' || true)"
  if [[ -n "$matches" ]]; then
    echo "error: tracked files contain assigned development credentials (pattern withheld)" >&2
    fail=1
  fi
}

key_access_secret="CF_ACCESS_CLIENT_SECRET"
key_api_token="SANTAIZI_API_TOKEN"
key_access_cookie="CF_AUTHORIZATION"
header_access_secret="CF-Access-Client-Secret"

scan "${key_access_secret}=.+"
scan "${key_api_token}=.+"
scan "${key_access_cookie}=.+"
scan "${header_access_secret}:[[:space:]]*[^[:space:]]+"

if [[ "$fail" -ne 0 ]]; then
  echo "error: remove secrets from git tracking; keep them in gitignored .env.local" >&2
  exit 1
fi
