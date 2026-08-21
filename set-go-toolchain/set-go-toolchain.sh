#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ge 1 ]]; then
  project="$1"
else
  project="/tedium/repo"
fi

if [[ ! -d "$project" ]]; then
  echo "no such directory: $project"
  exit 1
fi

while read f; do
  if ! grep toolchain "$f" >/dev/null; then
    sed -i 's/go 1.*/go 1.26.0\n\ntoolchain go1.26.6/' "$f"
  fi
done <<<$(find "$project" -name go.mod)
