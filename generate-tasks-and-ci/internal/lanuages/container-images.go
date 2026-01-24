package lanuages

import (
	"fmt"
	"path"
	"regexp"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
)

type ContainerImageProject struct {
	ProjectPath       string
	RelativePath      string
	ContainerFileName string
}

func FindContainerImageProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	imgManifestPaths, err := util.Find(
		projectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)Dockerfile$`),
			regexp.MustCompile(`(^|/)Containerfile$`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error searching for container image projects: %w", err)
	}

	for _, p := range imgManifestPaths {
		output = append(output, &ContainerImageProject{
			ProjectPath:       path.Join(projectPath, path.Dir(p)),
			RelativePath:      path.Dir(p),
			ContainerFileName: path.Base(p),
		})
	}

	return output, nil
}

func (p *ContainerImageProject) GetProjectPath() string {
	return p.ProjectPath
}

func (p *ContainerImageProject) GetRelativePath() string {
	return p.RelativePath
}

func (p *ContainerImageProject) AddTasks(taskFile *task.TaskFile) error {
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

func (p *ContainerImageProject) builderSetup() string {
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

func (p *ContainerImageProject) addRefsTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("imgrefs-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `
set -euo pipefail

if [[ -f .task-meta-imgrefs ]] && [[ ${CI+y} == "y" ]]; then
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

if ! grep ".task-meta-*" .gitignore >/dev/null 2>&1; then
  echo ".gitignore must include .task-meta-* to use the image builder tasks" >&2
  exit 1
fi

img_name=$( (grep "LABEL image.name=" ` + p.ContainerFileName + ` || echo) | tail -n 1 | cut -d '=' -f 2-)
img_registry=$( (grep "LABEL image.registry=" ` + p.ContainerFileName + ` || echo) | tail -n 1 | cut -d '=' -f 2-)

version=$(git describe --tags)
is_exact_tag=$(git describe --tags --exact-match >/dev/null 2>&1 && echo y || echo n)
major_version=$(echo "${version}" | cut -d '.' -f 1)
latest_version_overall=$(git tag -l | sort -V | tail -n 1)
latest_version_within_major=$(git tag -l | grep "^${major_version}" | sort -V | tail -n 1)

echo -n "" > .task-meta-imgrefs

if [[ ! -z "$img_name" ]]; then
  echo "localhost/${img_name}" >> .task-meta-imgrefs
  echo "localhost/${img_name}:${version}" >> .task-meta-imgrefs

  if [[ ! -z "$img_registry" ]] && [[ ${CI+y} == "y" ]]; then
    echo "${img_registry}/${img_name}:${version}" >> .task-meta-imgrefs

    if [[ "${is_exact_tag}" == "y" ]] && [[ "${version}" == "${latest_version_within_major}" ]]; then
      echo "${img_registry}/${img_name}:${major_version}" >> .task-meta-imgrefs
    fi

    if [[ "${is_exact_tag}" == "y" ]] && [[ "${version}" == "${latest_version_overall}" ]]; then
      echo "${img_registry}/${img_name}:latest" >> .task-meta-imgrefs
    fi
  fi
else
  echo "Warning: no image name label; image will not be tagged" >&2
fi

echo "Image refs:"
cat .task-meta-imgrefs | grep "." || echo "None"
`},
		},
	}

	return nil
}

func (p *ContainerImageProject) addBuildTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("imgbuild-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Dependencies: []string{
			fmt.Sprintf("imgrefs-%s", util.PathToSafeName(p.RelativePath)),
		},
		Commands: []task.Command{
			{Command: `
set -euo pipefail

` + p.builderSetup() + `

opts=(
	-f "` + p.ContainerFileName + `"
)

# Populate args if a file exists (Podman supports --build-arg-file, but Docker does not)
if [[ -f argfile.conf ]]; then
  while read arg; do
    k=$(cut -d = -f 1 <<<"$arg")
    v=$(cut -d = -f 2- <<<"$arg")
    opts+=("--build-arg" "$k=$v")
  done <<< "$(cat argfile.conf | grep -v '^#' | grep '=')"
fi

# First build to get visible logs
$builder build "${opts[@]}" .

# Second (cached) build to get the image ID
img=$($builder build "${opts[@]}" -q .)

if [[ -f .task-meta-imgrefs ]]; then
  cat .task-meta-imgrefs | while read tag; do
    $builder tag "$img" "${tag}"
    echo "Tagged ${tag}"
  done
fi
`},
		},
	}

	return nil
}

func (p *ContainerImageProject) addPushTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("imgpush-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Dependencies: []string{
			fmt.Sprintf("imgrefs-%s", util.PathToSafeName(p.RelativePath)),
		},
		Commands: []task.Command{
			{Command: `
set -euo pipefail

` + p.builderSetup() + `

if [[ -f .task-meta-imgrefs ]]; then
  cat .task-meta-imgrefs | (grep -v "^localhost" || :) | while read tag; do
    $builder push "${tag}"
    echo "Pushed ${tag}"
  done
else
  echo "No .task-meta-imgrefs file - nothing will be pushed"
  exit 1
fi
`},
		},
	}

	return nil
}
