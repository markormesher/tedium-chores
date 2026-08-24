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
if ! command -v buildah >/dev/null 2>&1; then
  echo "Buildah is not available" >&2
  exit 1
fi

buildah_opts=(
	--log-level debug
)

if command -v fuse-overlayfs >/dev/null 2>&1; then
  buildah_opts+=("--storage-opt" "overlay.mount_program=$(command -v fuse-overlayfs)")
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
set -x

` + p.builderSetup() + `

img_registry=$( (grep "LABEL image.registry=" ` + p.ContainerFileName + ` || echo) | tail -n 1 | cut -d '=' -f 2-)
img_name=$( (grep "LABEL image.name=" ` + p.ContainerFileName + ` || echo) | tail -n 1 | cut -d '=' -f 2-)

bud_opts=(
  --layers
  --timestamp 0
  --omit-history
  -f "` + p.ContainerFileName + `"
)

if [[ ! -z "${img_registry}" ]] && [[ ! -z "${img_name}" ]]; then
  bud_opts+=("--cache-to" "${img_registry}/${img_name}")
  bud_opts+=("--cache-from" "${img_registry}/${img_name}")
fi

if [[ -f argfile.conf ]]; then
  bud_opts+=("--build-arg-file" "argfile.conf")
fi

# first build to get visible logs
buildah "${buildah_opts[@]}" bud "${bud_opts[@]}"

# Second (cached) build to get the image ID
img=$(buildah "${buildah_opts[@]}" bud "${bud_opts[@]}" -q)

if [[ -f .task-meta-imgrefs ]]; then
  cat .task-meta-imgrefs | while read tag; do
    buildah "${buildah_opts[@]}" tag "$img" "${tag}"
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
    buildah "${buildah_opts[@]}" push "${tag}"
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
