package lanuages

import (
	"fmt"
	"path"
	"regexp"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
)

type SQLCProject struct {
	RelativePath string
}

func FindSQLCProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	sqlcGenPaths, err := util.Find(
		projectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)sqlc\.ya?ml`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error searching for SQLC projects: %w", err)
	}

	for _, p := range sqlcGenPaths {
		output = append(output, &SQLCProject{
			RelativePath: path.Dir(p),
		})
	}

	return output, nil
}

func (p *SQLCProject) GetRelativePath() string {
	return p.RelativePath
}

func (p *SQLCProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
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

func (p *SQLCProject) addGenTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("gen-sqlc-%s", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: `sqlc generate`},
		},
	}

	return nil
}
