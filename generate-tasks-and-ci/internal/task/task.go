package task

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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
	Directory    string    `yaml:"dir,omitempty"`
	Dependencies []string  `yaml:"deps,omitempty"`
	Sources      []string  `yaml:"sources,omitempty"`
	Generates    []string  `yaml:"generates,omitempty"`
	Internal     bool      `yaml:"internal,omitempty"`
	Commands     []Command `yaml:"cmds"`
}

type Command struct {
	Command string `yaml:"cmd,omitempty"`
	Task    string `yaml:"task,omitempty"`
}

func LoadTaskFile(path string) (*TaskFile, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error checking TaskFile path: %w", err)
	}

	taskfileContents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading TaskFile: %w", err)
	}

	var taskfile TaskFile
	decoder := yaml.NewDecoder(bytes.NewReader(taskfileContents))
	decoder.KnownFields(false)
	err = decoder.Decode(&taskfile)
	if err != nil {
		return nil, fmt.Errorf("error parsing TaskFile: %w", err)
	}

	return &taskfile, nil
}
