package main

import (
	"fmt"
	"path"
)

type BufProject struct {
	ProjectRelativePath string
}

func (p *BufProject) AddTasks(taskFile *TaskFile) error {
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

func (p *BufProject) addLintTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("lint-buf-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `buf lint`},
		},
	}

	return nil
}

func (p *BufProject) addGenTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("gen-buf-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `buf generate`},
		},
	}

	return nil
}
