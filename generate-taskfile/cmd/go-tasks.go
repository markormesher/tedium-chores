package main

import (
	"fmt"
	"path"
)

type GoProject struct {
	ProjectRelativePath string
}

func (p *GoProject) AddLintTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("lint-go-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `lint_diff=$(gofmt -e -s -d .)
if [[ ! -z "$lint_diff" ]]; then
  echo "Lint errors:"
  echo "$lint_diff"
  exit 1
fi`},
		},
	}

	taskFile.Tasks[name] = task

	if parentTask != nil {
		parentTask.Commands = append(parentTask.Commands, Command{Task: name})
	}

	return nil
}

func (p *GoProject) AddLintFixTask(taskFile *TaskFile, parentTask *Task) error {
	name := fmt.Sprintf("lint-fix-go-%s", pathToSafeName(p.ProjectRelativePath))
	task := &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `gofmt -s -w .`},
		},
	}

	taskFile.Tasks[name] = task

	if parentTask != nil {
		parentTask.Commands = append(parentTask.Commands, Command{Task: name})
	}

	return nil
}
