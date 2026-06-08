package ci

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

const ActionsCiFilePath = ".circleci/config.yml"

type ActionsConfig struct {
	Name string                      `yaml:"name,omitempty"`
	On   []string                    `yaml:"on"`
	Jobs map[string]ActionsJobConfig `yaml:"jobs"`
}

type ActionsJobConfig struct {
	RunsOn        string                    `yaml:"runs-on"`
	If            string                    `yaml:"if,omitempty"`
	Needs         []*regexp.Regexp          `yaml:"-"`
	ResolvedNeeds []string                  `yaml:"needs,omitempty"`
	Container     ActionsJobContainerConfig `yaml:"container,omitempty"`
	Steps         []ActionsJobStepConfig    `yaml:"steps"`
}

type ActionsJobContainerConfig struct {
	Image string `yaml:"image"`
}

type ActionsJobStepConfig struct {
	Name        string            `yaml:"name,omitempty"`
	Shell       string            `yaml:"shell,omitempty"`
	Environment map[string]string `yaml:"env,omitempty"`
	Uses        string            `yaml:"uses,omitempty"`
	Run         string            `yaml:"run,omitempty"`
}

func LoadActionsConfigIfPresent(path string) (*ActionsConfig, []byte, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil, nil
	} else if err != nil {
		return nil, nil, fmt.Errorf("error checking Actions config path: %w", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading Actions config: %w", err)
	}

	var config ActionsConfig
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(false)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, contents, fmt.Errorf("error parsing Actions config: %w", err)
	}

	return &config, contents, nil
}
