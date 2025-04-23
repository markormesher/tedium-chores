# Chore: Generate Taskfile

This [Tedium](https://github.com/markormesher/tedium) chore generates CI config file for CircleCI or Drone, depending on whether the project is public or private.

**Note** that this chore hardcodes assumptions that work for my projects but will not work for yours, such as a remote Podman server or my specific GHCR username. I'm entirely open to making that all configurable if there's a demand.

## Customisation

- `${language}_RUNTIME_PACKAGES` - specify extra packages to be installed (via `apt`) during lint and test steps for the given language.
  - e.g. `GO_RUNTIME_PACKAGES="libusb-1.0-0-dev"`
- `${language}_RUNTIME_ENV_${key}` - specify arbitrary extra environment variable to be inserted during lint and test steps for the given language.
  - e.g. `GO_RUNTIME_ENV_GOFLAGS="-foo=bar"` will add `export GOFLAGS="-foo=bar"` to lint and test steps for Go.
