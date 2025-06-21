package lanuages

import (
	"fmt"
	"path"
	"regexp"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
)

type GoProject struct {
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
			RelativePath: path.Dir(p),
		})
	}

	return output, nil
}

func (p *GoProject) GetRelativePath() string {
	return p.RelativePath
}

func (p *GoProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
		p.addCacheKeyTask,
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
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `sha256sum go.mod | awk '{print $1}' >> "{{.ROOT_DIR}}/.task-meta-cachekey-go"`},
			{Command: `if [[ -f go.sum ]]; then sha256sum go.sum | awk '{print $1}' >> "{{.ROOT_DIR}}/.task-meta-cachekey-go"; fi`},
		},
	}

	return nil
}

func (p *GoProject) addDepsTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("deps-go-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `go mod tidy && go mod download --json`},
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
		p.RelativePath,
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
