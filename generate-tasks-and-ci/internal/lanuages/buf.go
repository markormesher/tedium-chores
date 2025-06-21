package lanuages

import (
	"fmt"
	"path"
	"regexp"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
)

type BufProject struct {
	RelativePath string
}

func FindBufProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	bufGenPaths, err := util.Find(
		projectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)buf\.gen\.ya?ml`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error searching for Buf projects: %w", err)
	}

	for _, p := range bufGenPaths {
		output = append(output, &BufProject{
			RelativePath: path.Dir(p),
		})
	}

	return output, nil
}

func (p *BufProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
		p.addLintTask,
		p.addLintFixTask,
		p.addGenTask,
	}

	for _, f := range adders {
		err := f(taskFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *BufProject) GetRelativePath() string {
	return p.RelativePath
}

func (p *BufProject) addLintTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("lint-buf-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `buf format --diff --exit-code`},
			{Command: `buf lint`},
		},
	}

	return nil
}

func (p *BufProject) addLintFixTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("lintfix-buf-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `buf format --write`},
		},
	}

	return nil
}

func (p *BufProject) addGenTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("gen-buf-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `buf generate`},
		},
	}

	return nil
}
