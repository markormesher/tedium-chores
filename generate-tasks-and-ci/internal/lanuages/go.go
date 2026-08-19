package lanuages

import (
	"fmt"
	"path"
	"regexp"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
)

type GoProject struct {
	ParentPath   string
	ProjectPath  string
	RelativePath string
}

func FindGoProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	goModPaths, err := util.Find(
		projectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)go\.mod`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error searching for Go projects: %w", err)
	}

	for _, p := range goModPaths {
		output = append(output, &GoProject{
			ParentPath:   projectPath,
			ProjectPath:  path.Join(projectPath, path.Dir(p)),
			RelativePath: path.Dir(p),
		})
	}

	return output, nil
}

func (p *GoProject) GetProjectPath() string {
	return p.ProjectPath
}

func (p *GoProject) GetRelativePath() string {
	return p.RelativePath
}

func (p *GoProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
		p.addCacheKeyTask,
		p.addCacheLoadTask,
		p.addCacheSaveTask,
		p.addDepsTask,
		p.addLintTask,
		p.addLintFixTask,
		p.addTestTask,
	}

	for _, f := range adders {
		err := f(taskFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *GoProject) addCacheKeyTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("cachekey-go-%s", util.PathToSafeName(p.RelativePath))

	nonAlphanumeric := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	projectName := nonAlphanumeric.ReplaceAllString(path.Base(p.ParentPath), "")

	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Generates: []string{".task-meta-cache-key"},
		Commands: []task.Command{
			{
				Command: `
if [ ${CI:+1} ]; then
  PROJECT="` + projectName + `"

  DEPS_SHA=$(cat go.mod | sha256sum | awk '{ print $1 }')

  if [[ -f go.sum ]]; then
    LOCK_SHA=$(cat go.sum | sha256sum | awk '{ print $1 }')
  else
    LOCK_SHA="nolock"
  fi

  echo "${PROJECT}-go-v1/${DEPS_SHA}/${LOCK_SHA}" > .task-meta-cache-key
fi
`,
			},
		},
	}

	return nil
}

func (p *GoProject) addCacheLoadTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("cacheload-go-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Dependencies: []string{
			fmt.Sprintf("cachekey-go-%s", util.PathToSafeName(p.RelativePath)),
		},
		Commands: []task.Command{
			{Command: cacheLoadCommand()},
		},
	}

	return nil
}

func (p *GoProject) addCacheSaveTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("cachesave-go-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Dependencies: []string{
			fmt.Sprintf("cachekey-go-%s", util.PathToSafeName(p.RelativePath)),
		},
		Commands: []task.Command{
			{Command: `echo "$(go env GOMODCACHE) $(go env GOCACHE)" > .task-meta-cache-paths`},
			{Command: cacheSaveCommand()},
		},
	}

	return nil
}

func (p *GoProject) addDepsTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("deps-go-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `go mod download`},
			{Command: `(go tool || true) | (grep '\.' || true) | while read t; do go build -o /dev/null $t; done`},
		},
	}

	return nil
}

func (p *GoProject) addLintTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("lint-go-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `
exit_code=0

# gofmt
result=$(gofmt -e -s -d $(go list -f '{{ "{{.Dir}}" }}' ./... | grep -v /.go/ | grep -v /vendor/))
if [[ ! -z "$result" ]]; then
  echo "## gofmt:"
  echo "$result"
  exit_code=1
fi

# staticcheck
if grep staticcheck go.mod >/dev/null; then
  result=$(go tool staticcheck -checks inherit,+ST1003,+ST1016 ./... || true)
  if [[ ! -z "$result" ]]; then
    echo "## staticcheck:"
    echo "$result"
    exit_code=1
  fi
fi

# errcheck
if grep errcheck go.mod >/dev/null; then
  result=$(go tool errcheck -ignoregenerated ./... || true)
  if [[ ! -z "$result" ]]; then
    echo "## errcheck:"
    echo "$result"
    exit_code=1
  fi
fi

exit $exit_code
`},
		},
	}

	return nil
}

func (p *GoProject) addLintFixTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("lintfix-go-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `gofmt -s -w .`},
		},
	}

	return nil
}

func (p *GoProject) addTestTask(taskFile *task.TaskFile) error {
	testFiles, err := util.Find(
		p.ProjectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`.*_test\.go`),
		},
		[]*regexp.Regexp{},
	)
	if err != nil {
		return fmt.Errorf("error checking for Go test files: %w", err)
	}

	if len(testFiles) == 0 {
		return nil
	}

	name := fmt.Sprintf("test-go-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `go test ./...`},
		},
	}

	return nil
}
