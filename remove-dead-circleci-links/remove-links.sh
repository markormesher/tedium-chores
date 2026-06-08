#!/usr/bin/env bash
set -euo pipefail

cd /tedium/repo

if [[ -f README.md ]] && [[ ! -d .circleci ]]; then
  sed -i '/CircleCI/d' README.md
fi
