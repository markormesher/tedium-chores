package lanuages

import (
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
)

type Project interface {
	AddTasks(taskFile *task.TaskFile) error
	GetRelativePath() string
	GetProjectPath() string
}

type TaskAdder func(taskFile *task.TaskFile) error

type ProjectFinder func(projectPath string) ([]Project, error)
