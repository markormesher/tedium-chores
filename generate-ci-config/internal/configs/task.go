package configs

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// deliberately minimal representation of a taskfile with on the fields we need

type Taskfile struct {
	Tasks map[string]*Task `yaml:"tasks"`
}

type Task struct {
	Internal  bool   `yaml:"internal"`
	Directory string `yaml:"dir"`
}

func LoadTaskFile(path string) (*Taskfile, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error checking Taskfile path: %w", err)
	}

	taskfileContents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading Taskfile: %w", err)
	}

	var taskfile Taskfile
	decoder := yaml.NewDecoder(bytes.NewReader(taskfileContents))
	decoder.KnownFields(false)
	err = decoder.Decode(&taskfile)
	if err != nil {
		return nil, fmt.Errorf("error parsing Taskfile: %w", err)
	}

	return &taskfile, nil
}
