package main

import (
	"fmt"
	"path"
)

type ContainerImageProject struct {
	ContainerFilePath   string
	ProjectRelativePath string
}

func (p *ContainerImageProject) AddImageBuildTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("img-build-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `
# Podman or Docker?
if command -v podman >/dev/null 2>&1; then
  cmd=podmand
elif command -v docker >/dev/null 2>&1; then
  cmd=docker
else
  echo "Cannot find Podman or Docker installed - image will not be built" >&2
	exit 1
fi

# First build to get visible logs
podman build .

# Second (cached) build to get the image ID
img=$(podman build -q .)

img_name=$(podman inspect $img | jq -rc '.[0].Config.Labels["image.name"]')
img_registry=$(podman inspect $img | jq -rc '.[0].Config.Labels["image.registry"]')
if [[ ! -z  "$img_name" ]]; then
  podman tag "$img" "$img_name"

	if [[ ! -z "$img_registry" ]]; then
		podman tag "$img" "$img_registry/$img_name"
	fi
else
  echo "Warning: no image name detected; this image has not been labelled" >&2
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
