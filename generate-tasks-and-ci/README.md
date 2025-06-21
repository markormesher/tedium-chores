# Chore: Generate Tasks & CI Config

This [Tedium](https://github.com/markormesher/tedium) chore generates [Taskfile](https://taskfile.dev) and CI (either Drone or CircleCI) configs based on the contents of the repo. The two configs are generated in the same chore because CI is tighly-coupled to the Taskfile.

## Taskfile

This part of the chore generates a [Taskfile](https://taskfile.dev) with a hierarchy of tasks suitable for both local dev and remote CI in multi-language projects.

The nested task structure is generally **per-type -> per-language -> per-project**, with each layer functioning as follows:

- Per-type tasks, e.g. `lint` or `test`, include all tasks of their type. These tasks contain no real logic, they only include sub-tasks. These are intended to be run in local development environments, where all the languages and dependencies for a project are expected to be installed.
- Per-language tasks, e.g. `lint-go` or `test-js`, include all tasks of their type for a given language. These tasks also contain no real logic. These are intended to be run in CI environments, making it easy to run separate steps for each language.
- Per-project tasks, e.g. `lint-go-root` or `test-js-frontend`, contain the actual logic to run a given type of task, for a given language, within a specific project.

## Supported Languages / Tools

- [Buf](https://buf.build)
- Container images (via `Containerfile` or `Dockerfile`)
- Go
- [Goverter](https://github.com/jmattheis/goverter)
- JavaScript (incl. TypeScript)
- [sqlc](https://sqlc.dev)

## Supported Tasks

- `cachekey`
  - `cachekey-go`
    - _per-project tasks_
  - `cachekey-js`
    - _per-project tasks_
- `deps`
  - `deps-go`
    - _per-project tasks_
  - `deps-js`
    - _per-project tasks_
- `gen` _(code generation)_
  - `gen-buf`
    - _per-project tasks_
  - `gen-goverter`
    - _per-project tasks_
  - `gen-sqlc`
    - _per-project tasks_
- `lint`
  - `lint-buf`
    - _per-project tasks_
  - `lint-go`
    - _per-project tasks_
  - `lint-js`
    - _per-project tasks_
- `lintfix`
  - `lintfix-go`
    - _per-project tasks_
  - `lintfix-js`
    - _per-project tasks_
  - `lintfix-proto`
    - _per-project tasks_
- `test`
  - `test-go`
    - _per-project tasks_
  - `test-js`
    - _per-project tasks_
- `imgrefs`
  - _per-project tasks_
- `imgbuild`
  - _per-project tasks_
- `imgpush`
  - _per-project tasks_

Note that `img*` projects do not have a middle per-language level.

## CI Config

This part of the chore generates CI config file for CircleCI or Drone, depending on whether the project is public or private.

**Note** that this chore hardcodes assumptions that work for my projects but will not work for yours, such as a remote Podman server or my specific GHCR username. I'm entirely open to making that all configurable if there's a demand.

### Customisation

- `${language}_RUNTIME_PACKAGES` - specify extra packages to be installed (via `apt`) during lint and test steps for the given language.
  - e.g. `GO_RUNTIME_PACKAGES="libusb-1.0-0-dev"`
- `${language}_RUNTIME_ENV_${key}` - specify arbitrary extra environment variable to be inserted during lint and test steps for the given language.
  - e.g. `GO_RUNTIME_ENV_GOFLAGS="-foo=bar"` will add `export GOFLAGS="-foo=bar"` to lint and test steps for Go.
