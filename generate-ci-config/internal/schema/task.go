package schema

// deliberately minimal representation of a taskfile with on the fields we need

type TaskFile struct {
	Tasks map[string]*Task `yaml:"tasks"`
}

type Task struct {
	Internal bool `yaml:"internal,omitempty"`
}
