package schema

type DroneConfig struct {
	Kind     string              `yaml:"kind"`
	Type     string              `yaml:"type"`
	Name     string              `yaml:"name"`
	Metadata DroneMetadataConfig `yaml:"metadata"`
	Trigger  DroneTriggerConfig  `yaml:"trigger"`
	Steps    []DroneStepConfig   `yaml:"steps"`
}

type DroneMetadataConfig struct {
	Namespace string `yaml:"namespace"`
}

type DroneTriggerConfig struct {
	Event DroneTriggerEventConfig `yaml:"event"`
}

type DroneTriggerEventConfig struct {
	Include []string `yaml:"onclude,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type DroneStepConfig struct {
	Name         string            `yaml:"name"`
	Image        string            `yaml:"image"`
	ImagePull    string            `yaml:"pull"`
	Dependencies []string          `yaml:"depends_on,omitempty"`
	Environment  map[string]string `yaml:"environment,omitempty"`
	Commands     []string          `yaml:"commands"`
}

func GenerateDroneConfig(steps []*GenericCiStep) DroneConfig {
	config := DroneConfig{
		Kind: "pipeline",
		Type: "kubernetes",
		Name: "default",
		Metadata: DroneMetadataConfig{
			Namespace: "drone-ci",
		},
		Trigger: DroneTriggerConfig{
			Event: DroneTriggerEventConfig{
				Exclude: []string{
					"pull_request",
				},
			},
		},
		Steps: []DroneStepConfig{},
	}

	for _, step := range steps {
		config.Steps = append(config.Steps, DroneStepConfig{
			Name:         step.Name,
			Image:        step.Image,
			ImagePull:    "always",
			Dependencies: step.ResolvedDependencies,
			Environment:  step.Environment,
			Commands:     step.Commands,
		})
	}

	return config
}
