package lanuages

import (
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/log"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
)

var l = log.Logger

type Project interface {
	AddTasks(taskFile *task.TaskFile) error
	GetRelativePath() string
}

type TaskAdder func(taskFile *task.TaskFile) error

type ProjectFinder func(projectPath string) ([]Project, error)
