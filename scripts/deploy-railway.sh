#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
if [[ ! -f dist/index.html ]]; then
  echo 'Build the static site first: go run ./cmd/site build --data site/data --out dist' >&2
  exit 1
fi

# Stage only the public export, never repository credentials or raw run data.
mkdir -p .benchmark-data
staging=$(mktemp -d .benchmark-data/railway-upload.XXXXXX)
trap 'rm -rf "$staging"' EXIT
mkdir "$staging/public"
cp -R dist/. "$staging/public/"
rm -f "$staging/public/.kamski-generated"
cp deploy/railway/Dockerfile deploy/railway/default.conf.template "$staging/"
railway up "$staging" --path-as-root --no-gitignore "$@"
