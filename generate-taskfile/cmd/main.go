package main

import (
	"bytes"
	"flag"
	"log/slog"
	"os"
	"path"
	"regexp"
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

	// determine languages/sub-projects in the projects
	countSubProjects, subProjects := findSubProjects(projectPath)
	if countSubProjects == 0 {
		l.Error("No compatible sub-projects found in the project path")
		os.Exit(1)
	}

	// init output and parent tasks
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

	lintParentTask := Task{
		Commands: []Command{},
	}

	lintFixParentTask := Task{
		Commands: []Command{},
	}

	testParentTask := Task{
		Commands: []Command{},
	}

	generateParentTask := Task{
		Commands: []Command{},
	}

	containerImageBuildParentTask := Task{
		Commands: []Command{},
	}

	containerImagePushParentTask := Task{
		Commands: []Command{},
	}

	output.Tasks["lint"] = &lintParentTask
	output.Tasks["lint-fix"] = &lintFixParentTask
	output.Tasks["test"] = &testParentTask
	output.Tasks["gen"] = &generateParentTask
	output.Tasks["img-build"] = &containerImageBuildParentTask
	output.Tasks["img-push"] = &containerImagePushParentTask

	// TODO: package init tasks

	// lint tasks

	for _, p := range subProjects.GoProjects {
		err = p.AddLintTask(&output, &lintParentTask)
		if err != nil {
			l.Error("Error generating Go lint task", "error", err)
			os.Exit(1)
		}

		err = p.AddLintFixTask(&output, &lintFixParentTask)
		if err != nil {
			l.Error("Error generating Go lint-fix task", "error", err)
			os.Exit(1)
		}
	}

	for _, p := range subProjects.BufProjects {
		err = p.AddLintTask(&output, &lintParentTask)
		if err != nil {
			l.Error("Error generating Buf lint task", "error", err)
			os.Exit(1)
		}
	}

	// test tasks

	for _, p := range subProjects.GoProjects {
		err = p.AddTestTask(&output, &testParentTask)
		if err != nil {
			l.Error("Error generating Go test task", "error", err)
			os.Exit(1)
		}
	}

	// generate tasks

	for _, p := range subProjects.BufProjects {
		err = p.AddGenerateTask(&output, &generateParentTask)
		if err != nil {
			l.Error("Error generating Buf generate task", "error", err)
			os.Exit(1)
		}
	}

	// TODO: project build tasks

	// container image tasks

	for _, p := range subProjects.ContainerImageProjects {
		err = p.AddImageBuildTask(&output, &containerImageBuildParentTask)
		if err != nil {
			l.Error("Error generating image build task", "error", err)
			os.Exit(1)
		}

		err = p.AddImagePushTask(&output, &containerImagePushParentTask)
		if err != nil {
			l.Error("Error generating image push task", "error", err)
			os.Exit(1)
		}
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
		ContainerImageProjects: make([]*ContainerImageProject, 0),
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
		subProjects.ContainerImageProjects = append(subProjects.ContainerImageProjects, &ContainerImageProject{
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
