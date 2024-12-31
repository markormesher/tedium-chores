package configs

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type CircleConfig struct {
	Version   string                     `yaml:"version"`
	Jobs      map[string]CirlceJobConfig `yaml:"jobs"`
	Workflows CirlceWorkflowsConfig      `yaml:"workflows"`
}

type CirlceJobConfig struct {
	Docker []CircleJobDockerConfig `yaml:"docker"`
	Steps  []CirlceJobStepConfig   `yaml:"steps"`
}

type CircleJobDockerConfig struct {
	Image string `yaml:"image"`
}

type CirlceJobStepConfig struct {
	Checkout    CircleJobStepCheckoutConfig    `yaml:"checkout,omitempty"`
	Attach      CircleJobStepAttachConfig      `yaml:"attach_workspace,omitempty"`
	SetupDocker CircleJobStepSetupDockerConfig `yaml:"setup_remote_docker,omitempty"`
	Run         CircleJobStepRunConfig         `yaml:"run,omitempty"`
	Persist     CircleJobStepPersistConfig     `yaml:"persist_to_workspace,omitempty"`
}

type CircleJobStepCheckoutConfig struct {
	Path string `yaml:"path"`
}

type CircleJobStepAttachConfig struct {
	At string `yaml:"at"`
}

type CircleJobStepSetupDockerConfig struct {
	LayerCaching bool `yaml:"docker_layer_caching"`
}

type CircleJobStepRunConfig struct {
	Command     string            `yaml:"command"`
	Environment map[string]string `yaml:"environment,omitempty"`
}

type CircleJobStepPersistConfig struct {
	Root  string   `yaml:"root"`
	Paths []string `yaml:"paths"`
}

type CirlceWorkflowsConfig struct {
	Verison int                  `yaml:"version"`
	Main    CirlceWorkflowConfig `yaml:"main"`
}

type CirlceWorkflowConfig struct {
	Jobs []map[string]CirlceWorkflowJobConfig `yaml:"jobs"`
}

type CirlceWorkflowJobConfig struct {
	Dependencies []string                   `yaml:"requires"`
	Filter       CircleWorkflowFilterConfig `yaml:"filters,omitempty"`
}

type CircleWorkflowFilterConfig struct {
	Tags CirlceWorkflowFilterTagsConfig `yaml:"tags"`
}

type CirlceWorkflowFilterTagsConfig struct {
	Only string `yaml:"only"`
}

func GenerateCircleConfig(steps []*GenericCiStep) CircleConfig {
	config := CircleConfig{
		Version: "2.1",
		Jobs:    make(map[string]CirlceJobConfig, 0),
		Workflows: CirlceWorkflowsConfig{
			Verison: 2,
			Main: CirlceWorkflowConfig{
				Jobs: make([]map[string]CirlceWorkflowJobConfig, 0),
			},
		},
	}

	for _, step := range steps {
		// job
		job := CirlceJobConfig{
			Docker: []CircleJobDockerConfig{
				{Image: step.Image},
			},
			Steps: make([]CirlceJobStepConfig, 0),
		}

		if step.IsCheckoutStep {
			job.Steps = append(job.Steps, CirlceJobStepConfig{
				Checkout: CircleJobStepCheckoutConfig{
					Path: ".",
				},
			})
		} else {
			job.Steps = append(job.Steps, CirlceJobStepConfig{
				Attach: CircleJobStepAttachConfig{
					At: ".",
				},
			})
		}

		if step.NeedsDocker {
			job.Steps = append(job.Steps, CirlceJobStepConfig{
				SetupDocker: CircleJobStepSetupDockerConfig{
					LayerCaching: true,
				},
			})
		}

		if len(step.Commands) > 0 {
			job.Steps = append(job.Steps, CirlceJobStepConfig{
				Run: CircleJobStepRunConfig{
					Command:     strings.Join(step.Commands, "\n"),
					Environment: step.Environment,
				},
			})
		}

		if len(step.PersistPatterns) > 0 {
			job.Steps = append(job.Steps, CirlceJobStepConfig{
				Persist: CircleJobStepPersistConfig{
					Root:  ".",
					Paths: step.PersistPatterns,
				},
			})
		}

		config.Jobs[step.Name] = job

		// workflow
		workflowJob := CirlceWorkflowJobConfig{
			Dependencies: step.ResolvedDependencies,
			Filter: CircleWorkflowFilterConfig{
				Tags: CirlceWorkflowFilterTagsConfig{
					Only: "/.*/",
				},
			},
		}

		workflowJobWrapper := make(map[string]CirlceWorkflowJobConfig, 0)
		workflowJobWrapper[step.Name] = workflowJob
		config.Workflows.Main.Jobs = append(config.Workflows.Main.Jobs, workflowJobWrapper)
	}

	return config
}

func LoadCircleConfigIfPresent(path string) (*CircleConfig, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error checking Circle config path: %w", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading Circle config: %w", err)
	}

	var config CircleConfig
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(false)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("error parsing Circle config: %w", err)
	}

	return &config, nil
}
