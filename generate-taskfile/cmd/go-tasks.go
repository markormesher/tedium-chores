package main

import (
	"fmt"
	"path"
)

type GoProject struct {
	ProjectRelativePath string
}

func (p *GoProject) AddTasks(taskFile *TaskFile) error {
	adders := []TaskAdder{
		p.addLintTask,
		p.addLintFixTask,
		p.addTestTask,
	}

	for _, f := range adders {
		err := f(taskFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *GoProject) addLintTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("lint-go-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
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

	return nil
}

func (p *GoProject) addLintFixTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("lintfix-go-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `gofmt -s -w .`},
		},
	}

	return nil
}

func (p *GoProject) addTestTask(taskFile *TaskFile) error {
	name := fmt.Sprintf("test-go-%s", pathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []Command{
			{Command: `go test ./...`},
		},
	}

	return nil
}
