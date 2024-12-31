package lanuages

import (
	"fmt"
	"path"
	"regexp"

	"github.com/markormesher/tedium-chores/generate-taskfile/internal/task"
	"github.com/markormesher/tedium-chores/generate-taskfile/internal/util"
)

type GoProject struct {
	ProjectRelativePath string
}

func FindGoProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	projectPaths, err := util.Find(
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

	for _, p := range projectPaths {
		output = append(output, &GoProject{
			ProjectRelativePath: path.Dir(p),
		})
	}

	return output, nil
}

func (p *GoProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
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

func (p *GoProject) addLintTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("lint-go-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: `lint_diff=$(gofmt -e -s -d .)
if [[ ! -z "$lint_diff" ]]; then
  echo "Lint errors:"
  echo "$lint_diff"
  exit 1
fi`},
		},
	}

	return nil
}

func (p *GoProject) addLintFixTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("lintfix-go-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: `gofmt -s -w .`},
		},
	}

	return nil
}

func (p *GoProject) addTestTask(taskFile *task.TaskFile) error {
	testFiles, err := util.Find(
		p.ProjectRelativePath,
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

	name := fmt.Sprintf("test-go-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: `go test ./...`},
		},
	}

	return nil
}