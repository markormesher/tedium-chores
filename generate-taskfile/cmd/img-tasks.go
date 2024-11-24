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
  echo "Cannot find Podman or Docker installed - image will not be built" >&2
  exit 1
fi

img_name=$( (grep "LABEL image.name=" ` + p.ContainerFileName + ` || echo) | head -n 1 | cut -d '=' -f 2-)
img_registry=$( (grep "LABEL image.registry=" ` + p.ContainerFileName + ` || echo) | head -n 1 | cut -d '=' -f 2-)
if git describe --tags >/dev/null 2>&1; then
  version=":"$(git describe --tags)
else
  version=""
fi
`
}

func (p *ContainerImageProject) AddImageBuildTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("img-build-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `
set -euo pipefail

` + p.jobSetup() + `

# First build to get visible logs
$builder build -f ` + p.ContainerFileName + ` .

# Second (cached) build to get the image ID
img=$($builder build -q -f ` + p.ContainerFileName + ` .)

if [[ ! -z "$img_name" ]]; then
  $builder tag "$img" "localhost/${img_name}${version}"
  echo "Tagged localhost/${img_name}${version}"

  if [[ ! -z "$img_registry" ]]; then
    $builder tag "$img" "${img_registry}/${img_name}${version}"
    echo "Tagged ${img_registry}/${img_name}${version}"
  fi
else
  echo "Warning: no image name label; this image has not been tagged" >&2
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

if [[ "$img_name" == "" ]]; then
  echo "Cannot determine image name - image will not be pushed" >&2
  exit 1
elif [[ "$img_registry" == "" ]]; then
  echo "Cannot determine image registry - image will not be pushed" >&2
  exit 1
elif [[ "$version" == "" ]]; then
  echo "Cannot determine image version - image will not be pushed" >&2
  exit 1
else
  $builder push "${img_registry}/${img_name}${version}"
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
