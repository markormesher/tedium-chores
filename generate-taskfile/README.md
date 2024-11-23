# Chore: Generate Taskfile

This [Tedium](https://github.com/markormesher/tedium) chore generates a [Taskfile](https://taskfile.dev) for each project in a repo.

The supported/planned tasks are:

- Go
  - :hourglass: Install dependencies
  - :white_check_mark: Run linter
  - :white_check_mark: Apply lint fixes
  - :hourglass: Run tests
  - :hourglass: Build
- TypeScript
  - :hourglass: Install dependencies
  - :hourglass: Run linter
  - :hourglass: Apply lint fixes
  - :hourglass: Run tests
  - :hourglass: Build
- Container images (via `Containerfile` or `Dockerfile`)
  - :white_check_mark: Build and tag image
    - Note: depends on `image.name` and optional `image.regsitry` labels
  - :white_check_mark: Push image
