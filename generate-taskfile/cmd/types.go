package main

type SubProjectData struct {
	ContainerImageProjects []*ContainerImageProject
	GoProjects             []*GoProject
}

type TaskFile struct {
	Version string `yaml:"version"`

	Tasks map[string]*Task `yaml:"tasks"`
}

type Task struct {
	Directory string    `yaml:"dir,omitempty"`
	Commands  []Command `yaml:"cmds"`
}

type Command struct {
	Command string `yaml:"cmd,omitempty"`
	Task    string `yaml:"task,omitempty"`
}
