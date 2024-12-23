package main

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var jsonHandler = slog.NewJSONHandler(os.Stdout, nil)
var l = slog.New(jsonHandler)

func main() {
	// read config and validate it
	var projectPath string
	flag.StringVar(&projectPath, "project", "/tedium/repo", "Project path to target")
	flag.Parse()

	projectPath = strings.TrimRight(projectPath, "/")

	stat, err := os.Stat(projectPath)
	if err != nil {
		l.Error("Error stating project path", "error", err)
		os.Exit(1)
	}

	if !stat.IsDir() {
		l.Error("Project path doesn't exist or isn't a directory")
		os.Exit(1)
	}

	countSubProjects, subProjects := findSubProjects(projectPath)
	if countSubProjects == 0 {
		l.Error("No compatible sub-projects found in the project path")
		os.Exit(1)
	}

	output := TaskFile{
		Version: "3",
		Includes: map[string]*IncludeTarget{
			"local": {
				TaskFile: "taskfile.local.yml",
				Optional: true,
			},
		},
		Tasks: map[string]*Task{},
	}

	// layer 3 tasks

	for _, p := range subProjects.BufProjects {
		err = p.AddTasks(&output)
		if err != nil {
			l.Error("Error generating Buf tasks", "error", err)
			os.Exit(1)
		}
	}

	for _, p := range subProjects.GoProjects {
		err = p.AddTasks(&output)
		if err != nil {
			l.Error("Error generating Go tasks", "error", err)
			os.Exit(1)
		}
	}

	for _, p := range subProjects.ContainerImageProjects {
		err = p.AddTasks(&output)
		if err != nil {
			l.Error("Error generating img tasks", "error", err)
			os.Exit(1)
		}
	}

	// layer 1 and 2 tasks
	layer3Names := slices.Collect(maps.Keys(output.Tasks))
	for _, name := range layer3Names {
		nameChunks := strings.Split(name, "-")
		if len(nameChunks) < 3 {
			l.Error("Found invalid task name", "name", name)
			os.Exit(1)
		}

		layer1Name := nameChunks[0]
		layer2Name := fmt.Sprintf("%s-%s", nameChunks[0], nameChunks[1])

		if _, ok := output.Tasks[layer1Name]; !ok {
			output.Tasks[layer1Name] = &Task{}
		}

		if _, ok := output.Tasks[layer2Name]; !ok {
			output.Tasks[layer2Name] = &Task{}
		}

		output.Tasks[layer1Name].Commands = append(output.Tasks[layer1Name].Commands, Command{Task: name})
		output.Tasks[layer2Name].Commands = append(output.Tasks[layer2Name].Commands, Command{Task: name})
	}

	// clean up output

	multipleLineBreaks := regexp.MustCompile(`\n\n+`)
	blankLines := regexp.MustCompile(`^\s*$`)

	for t := range output.Tasks {
		if len(output.Tasks[t].Commands) == 0 {
			// remove empty tasks
			delete(output.Tasks, t)
			continue
		}

		for c := range output.Tasks[t].Commands {
			cmd := output.Tasks[t].Commands[c].Command
			cmd = strings.TrimSpace(cmd)
			cmd = blankLines.ReplaceAllString(cmd, "")
			cmd = multipleLineBreaks.ReplaceAllString(cmd, "\n\n")
			output.Tasks[t].Commands[c].Command = cmd
		}
	}

	// write output

	var outputBuffer bytes.Buffer
	encoder := yaml.NewEncoder(&outputBuffer)
	encoder.SetIndent(2)
	err = encoder.Encode(output)
	if err != nil {
		l.Error("Couldn't marshall output")
		os.Exit(1)
	}

	err = os.WriteFile(path.Join(projectPath, "taskfile.yml"), outputBuffer.Bytes(), 0644)
	if countSubProjects == 0 {
		l.Error("Error writing to taskfile", "error", err)
		os.Exit(1)
	}
}

func findSubProjects(projectPath string) (int, *SubProjectData) {
	countProjectsFound := 0
	subProjects := SubProjectData{
		ContainerImageProjects: make([]*ImgProject, 0),
		GoProjects:             make([]*GoProject, 0),
	}

	// containers

	containerImagePaths, err := find(
		projectPath,
		FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)Dockerfile`),
			regexp.MustCompile(`(^|/)Containerfile`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		l.Error("Error searching for Container image projects", "error", err)
		os.Exit(1)
	}

	for i := range containerImagePaths {
		countProjectsFound++
		subProjects.ContainerImageProjects = append(subProjects.ContainerImageProjects, &ImgProject{
			ContainerFileName:   path.Base(containerImagePaths[i]),
			ProjectRelativePath: path.Dir(containerImagePaths[i]),
		})
	}

	// buf

	bufProjectPaths, err := find(
		projectPath,
		FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)buf\.gen\.ya?ml`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		l.Error("Error searching for Buf projects", "error", err)
		os.Exit(1)
	}

	for i := range bufProjectPaths {
		countProjectsFound++
		subProjects.BufProjects = append(subProjects.BufProjects, &BufProject{
			ProjectRelativePath: path.Dir(bufProjectPaths[i]),
		})
	}

	// go

	goProjectPaths, err := find(
		projectPath,
		FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)go\.mod`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		l.Error("Error searching for Go projects", "error", err)
		os.Exit(1)
	}

	for i := range goProjectPaths {
		countProjectsFound++
		subProjects.GoProjects = append(subProjects.GoProjects, &GoProject{
			ProjectRelativePath: path.Dir(goProjectPaths[i]),
		})
	}

	return countProjectsFound, &subProjects
}
