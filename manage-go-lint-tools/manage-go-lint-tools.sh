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

shopt -s globstar
for gomod in "$project"/**/go.mod; do
  gomod_dir=$(dirname "$gomod")
  (
    # run everything in a subshell in this project's directory
    cd "$gomod_dir"

    # check that we support go tools
    project_version=$(go mod edit -json | jq -rc ".Go")
    min_version="1.24.0"
    if ! echo -e "${min_version}\n${project_version}" | sort -C -V; then
      echo "project is not using Go >= ${min_version} - cannot install tools"
      exit 0
    fi

    # install tools
    if ! go tool | grep "staticcheck" >/dev/null; then
      go get -tool honnef.co/go/tools/cmd/staticcheck@latest
    fi

    if ! go tool | grep "errcheck" >/dev/null; then
      go get -tool github.com/kisielk/errcheck@latest
    fi
  )
done
