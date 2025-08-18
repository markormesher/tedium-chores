#!/usr/bin/env bash
set -euo pipefail

bookworm_version="12.11"
trixie_version="13.0"

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
  sed -i -E "s/debian:bookworm(\\-slim)?(@sha[:a-z0-9+]+)?/debian:${bookworm_version}\\1/g" "$f"
  sed -i -E "s/debian:trixie(\\-slim)?(@sha[:a-z0-9+]+)?/debian:${trixie_version}\\1/g" "$f"
done <<<$(find "$project" -name Containerfile -or -name Dockerfile -or -path '*.circleci/config.yml' -or -name .drone.yml)
