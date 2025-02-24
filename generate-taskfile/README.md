# Chore: Generate Taskfile

This [Tedium](https://github.com/markormesher/tedium) chore generates a [Taskfile](https://taskfile.dev) with a hierarchy of tasks suitable for both local dev and remote CI in multi-language projects.

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
