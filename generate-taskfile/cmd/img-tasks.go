package main

import (
	"fmt"
	"path"
)

type ImgProject struct {
	ContainerFileName   string
	ProjectRelativePath string
}

func (p *ImgProject) AddTasks(taskFile *TaskFile) error {
	adders := []TaskAdder{
		p.addRefsTask,
		p.addBuildTask,
		p.addPushTask,
	}

	for _, f := range adders {
		err := f(taskFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *ImgProject) builderSetup() string {
	return `
if command -v podman >/dev/null 2>&1; then
  # Podman for building locally or in Tatsu CI
  builder=podman
elif command -v docker >/dev/null 2>&1; then
  # Docker for building in Circle CI
  builder=docker
else
  echo "Cannot find Podman or Docker" >&2
  exit 1
fi
`
}

func (p *ImgProject) addRefsTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("imgrefs-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `
set -euo pipefail

if [[ -f .imgrefs ]] && [[ ${CI+y} == "y" ]]; then
  echo "Skipping re-computing tags"
  exit 0
fi

if ! command -v git >/dev/null 2>&1; then
  echo "Cannot find git" >&2
  exit 1
fi

if ! git describe --tags >/dev/null 2>&1; then
  echo "No git tags to descibe" >&2
  exit 1
fi

if ! grep ".imgrefs" .gitignore >/dev/null 2>&1; then
  echo ".gitignore must include .imgrefs to use the image builder tasks" >&2
  exit 1
fi

img_name=$( (grep "LABEL image.name=" ` + p.ContainerFileName + ` || echo) | head -n 1 | cut -d '=' -f 2-)
img_registry=$( (grep "LABEL image.registry=" ` + p.ContainerFileName + ` || echo) | head -n 1 | cut -d '=' -f 2-)

version=$(git describe --tags)
is_exact_tag=$(git describe --tags --exact-match >/dev/null 2>&1 && echo y || echo n)
major_version=$(echo "${version}" | cut -d '.' -f 1)
latest_version_overall=$(git tag -l | sort -r -V | head -n 1)
latest_version_within_major=$(git tag -l | grep "^${major_version}" | sort -r -V | head -n 1)

echo -n "" > .imgrefs

if [[ ! -z "$img_name" ]]; then
  echo "localhost/${img_name}" >> .imgrefs
  echo "localhost/${img_name}:${version}" >> .imgrefs

  if [[ ! -z "$img_registry" ]] && [[ ${CI+y} == "y" ]]; then
    echo "${img_registry}/${img_name}:${version}" >> .imgrefs

    if [[ "${is_exact_tag}" == "y" ]] && [[ "${version}" == "${latest_version_within_major}" ]]; then
      echo "${img_registry}/${img_name}:${major_version}" >> .imgrefs
    fi

    if [[ "${is_exact_tag}" == "y" ]] && [[ "${version}" == "${latest_version_overall}" ]]; then
      echo "${img_registry}/${img_name}:latest" >> .imgrefs
    fi
  fi
else
  echo "Warning: no image name label; image will not be tagged" >&2
fi

echo "Image refs:"
cat .imgrefs | grep "." || echo "None"
`},
		},
	}

	return nil
}

func (p *ImgProject) addBuildTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("imgbuild-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Dependencies: []string{
			fmt.Sprintf("imgrefs-%s", pathToSafeName(p.ProjectRelativePath)),
		},
		Commands: []Command{
			{Command: `
set -euo pipefail

` + p.builderSetup() + `

# First build to get visible logs
$builder build -f ` + p.ContainerFileName + ` .

# Second (cached) build to get the image ID
img=$($builder build -q -f ` + p.ContainerFileName + ` .)

if [[ -f .imgrefs ]]; then
  cat .imgrefs | while read tag; do
    $builder tag "$img" "${tag}"
    echo "Tagged ${tag}"
  done
fi
`},
		},
	}

	return nil
}

func (p *ImgProject) addPushTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("imgpush-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Dependencies: []string{
			fmt.Sprintf("imgrefs-%s", pathToSafeName(p.ProjectRelativePath)),
		},
		Commands: []Command{
			{Command: `
set -euo pipefail

` + p.builderSetup() + `

if [[ -f .imgrefs ]]; then
  cat .imgrefs | (grep -v "^localhost" || :) | while read tag; do
    $builder push "${tag}"
    echo "Pushed ${tag}"
  done
else
  echo "No .imgrefs file - nothing will be pushed"
	exit 1
fi
`},
		},
	}

	return nil
}
