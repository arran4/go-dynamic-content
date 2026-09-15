#!/usr/bin/env bash
set -euo pipefail

REF="${TARGET_REF:-${GITHUB_REF:-}}"
SHA="${TARGET_SHA:-${GITHUB_SHA:-}}"

if [[ "$REF" != "refs/heads/main" ]]; then
  echo "Error: Manual release preparation must run on refs/heads/main, got $REF"
  exit 1
fi

git fetch origin main --quiet
MAIN_SHA=$(git rev-parse origin/main)

if [[ "$MAIN_SHA" != "$SHA" ]]; then
  echo "Error: Requested release against $SHA but origin/main is at $MAIN_SHA"
  exit 1
fi
