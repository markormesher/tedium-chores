package main

import (
	"fmt"
	"path"
)

type BufProject struct {
	ProjectRelativePath string
}

func (p *BufProject) AddLintTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("lint-buf-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `buf lint`},
		},
	}

	taskFile.Tasks[name] = task

	if parentTask != nil {
		parentTask.Commands = append(parentTask.Commands, Command{Task: name})
	}

	return nil
}

func (p *BufProject) AddGenerateTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("gen-buf-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `buf generate`},
		},
	}

	taskFile.Tasks[name] = task

	if parentTask != nil {
		parentTask.Commands = append(parentTask.Commands, Command{Task: name})
	}

	return nil
}
