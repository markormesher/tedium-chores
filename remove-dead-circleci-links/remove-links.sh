#!/usr/bin/env bash
set -euo pipefail

cd ~/public-dev/tfl-to-mqtt/

if [[ -f README.md ]] && [[ ! -d .circleci ]]; then
  sed -i '/CircleCI/d' README.md
fi
