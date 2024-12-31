package lanuages

import (
	"fmt"
	"path"
	"regexp"

	"github.com/markormesher/tedium-chores/generate-taskfile/internal/task"
	"github.com/markormesher/tedium-chores/generate-taskfile/internal/util"
)

type BufProject struct {
	ProjectRelativePath string
}

func FindBufProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	projectPaths, err := util.Find(
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

	for _, p := range projectPaths {
		output = append(output, &BufProject{
			ProjectRelativePath: path.Dir(p),
		})
	}

	return output, nil
}

func (p *BufProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
		p.addLintTask,
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

func (p *BufProject) addLintTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("lint-buf-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: `buf lint`},
		},
	}

	return nil
}

func (p *BufProject) addGenTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("gen-buf-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: `buf generate`},
		},
	}

	return nil
}
