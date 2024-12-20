package main

type SubProjectData struct {
	ContainerImageProjects []*ContainerImageProject
	GoProjects             []*GoProject
}

type TaskFile struct {
	Version  string                    `yaml:"version"`
	Includes map[string]*IncludeTarget `yaml:"includes"`
	Tasks    map[string]*Task          `yaml:"tasks"`
}

type IncludeTarget struct {
	TaskFile string `yaml:"taskfile"`
	Optional bool   `yaml:"optional"`
}

type Task struct {
	Directory string    `yaml:"dir,omitempty"`
	Commands  []Command `yaml:"cmds"`
}

type Command struct {
	Command string `yaml:"cmd,omitempty"`
	Task    string `yaml:"task,omitempty"`
}
