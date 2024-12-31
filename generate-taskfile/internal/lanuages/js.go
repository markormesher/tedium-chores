package lanuages

import "github.com/markormesher/tedium-chores/generate-taskfile/internal/task"

// placholder

type JsProject struct {
	ProjectRelativePath string
}

func FindJsProjects(projectPath string) ([]Project, error) {
	output := []Project{}
	return output, nil
}

func (p *JsProject) AddTasks(taskFile *task.TaskFile) error {
	return nil
}
