package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/lanuages"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"gopkg.in/yaml.v3"
)

func updateTaskfile(projectPath string) {
	// output skeleton - this will be mutated by each language to add tasks
	taskFile := task.TaskFile{
		Version: "3",
		Includes: map[string]*task.IncludeTarget{
			"local": {
				TaskFile: "taskfile.local.yml",
				Optional: true,
			},
		},
		Tasks: map[string]*task.Task{},
	}

	// collect projects and generate layer-3 tasks
	allProjects := []lanuages.Project{}
	projectFinders := []lanuages.ProjectFinder{
		lanuages.FindBufProjects,
		lanuages.FindContainerImageProjects,
		lanuages.FindGoProjects,
		lanuages.FindGoverterProjects,
		lanuages.FindJSProjects,
		lanuages.FindSQLCProjects,
	}

	for _, finder := range projectFinders {
		projects, err := finder(projectPath)
		if err != nil {
			l.Error("error finding projects", "error", err)
			os.Exit(1)
		}
		allProjects = append(allProjects, projects...)
	}

	for _, p := range allProjects {
		err := updateGitignore(path.Join(projectPath, p.GetRelativePath()))
		if err != nil {
			l.Error("error updating .gitignore", "error", err)
			os.Exit(1)
		}

		err = p.AddTasks(&taskFile)
		if err != nil {
			l.Error("error adding tasks", "error", err)
			os.Exit(1)
		}
	}

	// collect names of layer-3 tasks that will be exposed
	layer3Names := make([]string, 0)
	for name, task := range taskFile.Tasks {
		if !task.Internal {
			layer3Names = append(layer3Names, name)
		}
	}

	// sort names to keep output ordering consistent
	slices.Sort(layer3Names)

	// generate layer-1 and layer-2 tasks
	for _, name := range layer3Names {
		nameChunks := strings.Split(name, "-")

		// all tasks have a layer-1 parent
		layer1Name := nameChunks[0]
		if _, ok := taskFile.Tasks[layer1Name]; !ok {
			taskFile.Tasks[layer1Name] = &task.Task{}
		}
		taskFile.Tasks[layer1Name].Commands = append(taskFile.Tasks[layer1Name].Commands, task.Command{Task: name})

		// not all tasks have layer-2 parent
		if len(nameChunks) > 2 {
			layer2Name := fmt.Sprintf("%s-%s", nameChunks[0], nameChunks[1])
			if _, ok := taskFile.Tasks[layer2Name]; !ok {
				taskFile.Tasks[layer2Name] = &task.Task{}
			}
			taskFile.Tasks[layer2Name].Commands = append(taskFile.Tasks[layer2Name].Commands, task.Command{Task: name})
		}
	}

	// one-off changes for specific tasks
	for name, t := range taskFile.Tasks {
		if name == "cachekey" {
			t.Commands = append([]task.Command{
				{Command: `rm -f "{{.ROOT_DIR}}/.task-meta-cachekey"*`},
			}, t.Commands...)
		}
	}

	// clean up output

	multipleLineBreaks := regexp.MustCompile(`\n\n+`)
	blankLines := regexp.MustCompile(`^\s*$`)

	for t := range taskFile.Tasks {
		if len(taskFile.Tasks[t].Commands) == 0 {
			// remove empty tasks
			delete(taskFile.Tasks, t)
			continue
		}

		for c := range taskFile.Tasks[t].Commands {
			cmd := taskFile.Tasks[t].Commands[c].Command
			cmd = strings.TrimSpace(cmd)
			cmd = blankLines.ReplaceAllString(cmd, "")
			cmd = multipleLineBreaks.ReplaceAllString(cmd, "\n\n")
			taskFile.Tasks[t].Commands[c].Command = cmd
		}
	}

	// write output

	var outputBuffer bytes.Buffer
	encoder := yaml.NewEncoder(&outputBuffer)
	encoder.SetIndent(2)
	err := encoder.Encode(taskFile)
	if err != nil {
		l.Error("Couldn't marshall output")
		os.Exit(1)
	}

	handleWriteError := func(err error) {
		if err != nil {
			l.Error("Error writing to taskfile", "error", err)
			os.Exit(1)
		}
	}

	outputPath := path.Join(projectPath, "taskfile.yml")
	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	handleWriteError(err)
	defer func() {
		_ = outputFile.Close()
	}()

	_, err = outputFile.WriteString("# This file is maintained by Tedium - manual edits will be overwritten!\n\n")
	handleWriteError(err)

	_, err = outputFile.Write(outputBuffer.Bytes())
	handleWriteError(err)
}

func updateGitignore(projectPath string) error {
	gitignorePath := path.Join(projectPath, ".gitignore")
	_, statErr := os.Stat(gitignorePath)

	var contents string
	if errors.Is(statErr, os.ErrNotExist) {
		contents = ""
	} else {
		contentsRaw, err := os.ReadFile(gitignorePath)
		if err != nil {
			return fmt.Errorf("error reading gitignore: %w", err)
		}
		contents = string(contentsRaw)
	}

	lines := strings.Split(contents, "\n")
	replaced := false
	seen := false
	for i, l := range lines {
		if l == ".imgrefs" {
			lines[i] = ".task-meta-*"
			replaced = true
			break
		}

		if l == ".task-meta-*" {
			seen = true
			break
		}
	}

	if !seen && !replaced {
		lines = append(lines, ".task-meta-*")
	}

	contents = strings.Join(lines, "\n")
	err := os.WriteFile(gitignorePath, []byte(contents), 0644)
	if err != nil {
		return fmt.Errorf("error writing gitignore: %w", err)
	}

	return nil
}
