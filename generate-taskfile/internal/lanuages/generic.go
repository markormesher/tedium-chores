package lanuages

import "github.com/markormesher/tedium-chores/generate-taskfile/internal/task"

type Project interface {
	AddTasks(taskFile *task.TaskFile) error
	GetRelativePath() string
}

type TaskAdder func(taskFile *task.TaskFile) error

type ProjectFinder func(projectPath string) ([]Project, error)
