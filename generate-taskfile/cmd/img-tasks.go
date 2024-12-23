package main

import (
	"fmt"
	"path"
)

type ContainerImageProject struct {
	ContainerFileName   string
	ProjectRelativePath string
}

func (p *ContainerImageProject) jobSetup() string {
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

func (p *ContainerImageProject) AddImageTagsTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("img-tags-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `
set -euo pipefail

if [[ -f .image-tags ]] && [[ ${CI+y} == "y" ]]; then
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

if ! grep ".image-tags" .gitignore >/dev/null 2>&1; then
  echo ".gitignore must include .image-tags to use the image builder tasks" >&2
  exit 1
fi

img_name=$( (grep "LABEL image.name=" ` + p.ContainerFileName + ` || echo) | head -n 1 | cut -d '=' -f 2-)
img_registry=$( (grep "LABEL image.registry=" ` + p.ContainerFileName + ` || echo) | head -n 1 | cut -d '=' -f 2-)

version=$(git describe --tags)
is_exact_tag=$(git describe --tags --exact-match >/dev/null 2>&1 && echo y || echo n)
major_version=$(echo "${version}" | cut -d '.' -f 1)
latest_version_overall=$(git tag -l | sort -r -V | head -n 1)
latest_version_within_major=$(git tag -l | grep "^${major_version}" | sort -r -V | head -n 1)

echo -n "" > .image-tags

if [[ ! -z "$img_name" ]]; then
  echo "localhost/${img_name}" >> .image-tags
  echo "localhost/${img_name}:${version}" >> .image-tags

  if [[ ! -z "$img_registry" ]] && [[ ${CI+y} == "y" ]]; then
    echo "${img_registry}/${img_name}:${version}" >> .image-tags

    if [[ "${is_exact_tag}" == "y" ]] && [[ "${version}" == "${latest_version_within_major}" ]]; then
      echo "${img_registry}/${img_name}:${major_version}" >> .image-tags
    fi

    if [[ "${is_exact_tag}" == "y" ]] && [[ "${version}" == "${latest_version_overall}" ]]; then
      echo "${img_registry}/${img_name}:latest" >> .image-tags
    fi
  fi
else
  echo "Warning: no image name label; image will not be tagged" >&2
fi

echo "Image tags:"
cat .image-tags
`},
		},
	}

	taskFile.Tasks[name] = task

	if parentTask != nil {
		parentTask.Commands = append(parentTask.Commands, Command{Task: name})
	}

	return nil
}

func (p *ContainerImageProject) AddImageBuildTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("img-build-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Dependencies: []string{
			fmt.Sprintf("img-tags-%s", pathToSafeName(p.ProjectRelativePath)),
		},
		Commands: []Command{
			{Command: `
set -euo pipefail

` + p.jobSetup() + `

# First build to get visible logs
$builder build -f ` + p.ContainerFileName + ` .

# Second (cached) build to get the image ID
img=$($builder build -q -f ` + p.ContainerFileName + ` .)

if [[ -f .image-tags ]]; then
  cat .image-tags | while read tag; do
    $builder tag "$img" "${tag}"
    echo "Tagged ${tag}"
  done
fi
`},
		},
	}

	taskFile.Tasks[name] = task

	if parentTask != nil {
		parentTask.Commands = append(parentTask.Commands, Command{Task: name})
	}

	return nil
}

func (p *ContainerImageProject) AddImagePushTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("img-push-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `
set -euo pipefail

` + p.jobSetup() + `

if [[ -f .image-tags ]]; then
  cat .image-tags | (grep -v "^localhost" || :) | while read tag; do
    $builder push "${tag}"
    echo "Pushed ${tag}"
  done
fi
`},
		},
	}

	taskFile.Tasks[name] = task

	if parentTask != nil {
		parentTask.Commands = append(parentTask.Commands, Command{Task: name})
	}

	return nil
}
