package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/ci"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
	"gopkg.in/yaml.v3"
)

// ResourceSet is a utility type to store the container image references used for various steps.
type ResourceSet struct {
	bufStepImage  string
	goStepImage   string
	imgStepImage  string
	jsStepImage   string
	sqlcStepImage string
	utilStepImage string

	ciResourcesAction    string
	ciResourcesActionTag string
}

func deleteOldCIConfigs(projectPath string) {
	projectPath = strings.TrimRight(projectPath, "/")

	projectPathExists, err := util.DirExists(projectPath)
	if err != nil {
		slog.Error("error checking whether project path exists", "error", err)
		os.Exit(1)
	}

	if !projectPathExists {
		slog.Error("project path does not exist", "path", projectPath)
		os.Exit(1)
	}

	oldPaths := []string{".circleci", ".drone.yml"}
	for _, p := range oldPaths {
		err := os.RemoveAll(path.Join(projectPath, p))
		if err != nil {
			slog.Error("error deleting old CI config", "path", p, "error", err)
			os.Exit(1)
		}
	}
}

func updateCIConfig(projectPath string) {
	projectPath = strings.TrimRight(projectPath, "/")

	projectPathExists, err := util.DirExists(projectPath)
	if err != nil {
		slog.Error("error checking whether project path exists", "error", err)
		os.Exit(1)
	}

	if !projectPathExists {
		slog.Error("project path does not exist", "path", projectPath)
		os.Exit(1)
	}

	privateGitDomain := os.Getenv("PRIVATE_GIT_DOMAIN")

	// define the output path based on the repo type
	outputPath := ""
	if privateGitDomain == "" {
		outputPath = path.Join(projectPath, ".github/workflows/ci.yml")
	} else {
		outputPath = path.Join(projectPath, ".forgejo/workflows/ci.yml")
	}

	// read taskfile
	taskfilePath := path.Join(projectPath, "taskfile.yml")
	taskfile, err := task.LoadTaskFile(taskfilePath)
	if err != nil {
		slog.Error("error loading Taskfile", "error", err)
		os.Exit(1)
	}
	if taskfile == nil {
		slog.Warn("no taskfile in this repo - skipping")
		os.Exit(0)
	}

	taskNames := []string{}
	for name, task := range taskfile.Tasks {
		if !task.Internal {
			taskNames = append(taskNames, name)
		}
	}

	// extract resource versions from existing CI files if possible
	resourceSet := ResourceSet{}
	oldConfig, oldConfigRaw, err := ci.LoadActionsConfigIfPresent(outputPath)
	if err != nil {
		slog.Warn("error reading existing config - continuing without it", "error", err)
	}
	if oldConfig != nil {
		resourceSet = extractResourcesFromConfig(*oldConfig, oldConfigRaw)
	}
	resourceSet.populateMissingResources(privateGitDomain)

	// init new config
	newConfig := ci.ActionsConfig{
		Name: "CI",
		On:   []string{"push"},
		Jobs: map[string]ci.ActionsJobConfig{},
	}

	// generic tasks
	newConfig.Jobs["ci-all"] = ci.ActionsJobConfig{
		RunsOn: "ubuntu-latest",
		If:     "always()",
		Container: ci.ActionsJobContainerConfig{
			Image: resourceSet.utilStepImage,
		},
		Needs: []*regexp.Regexp{
			regexp.MustCompile(`check\-.*`),
			regexp.MustCompile(`img\-*`),
		},
		Steps: []ci.ActionsJobStepConfig{
			{
				Run: `
results=$(grep '"result":' <<<'${{ toJson(needs) }}')
if echo "$results" | grep -q "failure\|cancelled\|skipped"; then
  echo "One or more jobs failed, were cancelled, or were skipped" >&2
  exit 1
fi
echo "All jobs passed"
`,
			},
		},
	}

	// language check tasks
	langs := []string{"buf", "go", "js", "sqlc"}
	langTasks := []string{"deps", "lint", "test"}
	for _, lang := range langs {
		// TODO: consider RUNTIME_ENV and RUNTIME_PACKAGES

		image, err := getImageForLanguageTask(resourceSet, lang)
		if err != nil {
			slog.Error("unable to get image to language task", "error", err)
			os.Exit(1)
		}

		job := ci.ActionsJobConfig{
			RunsOn: "ubuntu-latest",
			Container: ci.ActionsJobContainerConfig{
				Image: image,
			},
			Steps: []ci.ActionsJobStepConfig{
				{Uses: resourceSet.ciResourcesAction},
			},
		}

		// handle special cases
		if lang == "js" {
			job.Steps = append(job.Steps, ci.ActionsJobStepConfig{
				Run: "npm install -g --force yarn pnpm",
			})
		}

		langHasTasks := false
		for _, langTask := range langTasks {
			taskName := fmt.Sprintf("%s-%s", langTask, lang)
			if slices.Contains(taskNames, taskName) {
				langHasTasks = true
				job.Steps = append(job.Steps, ci.ActionsJobStepConfig{
					Run: fmt.Sprintf("./task %s", taskName),
				})
			}
		}

		if langHasTasks {
			newConfig.Jobs["check-"+lang] = job
		}
	}

	// container image tasks
	if slices.Contains(taskNames, "imgrefs") {
		job := ci.ActionsJobConfig{
			RunsOn: "ubuntu-latest",
			Needs: []*regexp.Regexp{
				regexp.MustCompile(`^check\-.*`),
			},
			Steps: []ci.ActionsJobStepConfig{
				{Uses: resourceSet.ciResourcesAction},
			},
		}

		// login stage
		if privateGitDomain == "" {
			job.Steps = append(job.Steps, ci.ActionsJobStepConfig{
				Run: `buildah login ghcr.io -u "${{ github.actor }}" -p "${{ github.token }}"`,
			})
		} else {
			job.Steps = append(job.Steps, ci.ActionsJobStepConfig{
				Run: fmt.Sprintf(`buildah login "%s" -u ci -p "${{ secrets.PACKAGE_PUBLISH_TOKEN }}"`, privateGitDomain),
			})
		}

		for _, t := range []string{"imgrefs", "imgbuild", "imgpush"} {
			job.Steps = append(job.Steps, ci.ActionsJobStepConfig{
				Run: fmt.Sprintf("./task %s", t),
			})
		}

		newConfig.Jobs["img-build-push"] = job
	}

	// resolve needs
	allJobsNames := slices.Collect(maps.Keys(newConfig.Jobs))
	for name, job := range newConfig.Jobs {
		resolvedNeeds := util.MatchingStrings(allJobsNames, job.Needs)
		job.ResolvedNeeds = resolvedNeeds
		newConfig.Jobs[name] = job
	}

	// write out the actual config
	var outputBuffer bytes.Buffer
	encoder := yaml.NewEncoder(&outputBuffer)
	encoder.SetIndent(2)
	err = encoder.Encode(newConfig)
	if err != nil {
		slog.Error("couldn't encode output")
		os.Exit(1)
	}

	// post-process lines
	outputLines := []string{
		"# This file is maintained by Tedium - manual edits will be overwritten!",
	}
	for line := range strings.SplitSeq(outputBuffer.String(), "\n") {
		// restore renovate actions versions comments if applicable
		if strings.Contains(line, resourceSet.ciResourcesAction) && resourceSet.ciResourcesActionTag != "" {
			line = line + " # " + resourceSet.ciResourcesActionTag
		}

		outputLines = append(outputLines, line)
	}
	output := strings.Join(outputLines, "\n")

	handleWriteError := func(err error) {
		if err != nil {
			slog.Error("error writing to CI config", "error", err)
			os.Exit(1)
		}
	}

	err = os.MkdirAll(path.Dir(outputPath), 0755)
	handleWriteError(err)

	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	handleWriteError(err)
	defer func() {
		err := outputFile.Close()
		handleWriteError(err)
	}()

	_, err = outputFile.WriteString(output)
	handleWriteError(err)
}

func getImageForLanguageTask(imageSet ResourceSet, lang string) (string, error) {
	switch lang {
	case "buf":
		return imageSet.bufStepImage, nil
	case "go":
		return imageSet.goStepImage, nil
	case "js":
		return imageSet.jsStepImage, nil
	case "sqlc":
		return imageSet.sqlcStepImage, nil
	default:
		return "", fmt.Errorf("unsupported language '%s'", lang)
	}
}

func extractResourcesFromConfig(config ci.ActionsConfig, rawConfig []byte) ResourceSet {
	output := ResourceSet{}

	for _, job := range config.Jobs {
		image := job.Container.Image
		switch {
		case strings.Contains(image, "bufbuild"):
			output.bufStepImage = image
		case strings.Contains(image, "busybox"):
			output.utilStepImage = image
		case strings.Contains(image, "golang"):
			output.goStepImage = image
		case strings.Contains(image, "node"):
			output.jsStepImage = image
		case strings.Contains(image, "podman"):
			output.imgStepImage = image
		case strings.Contains(image, "sqlc"):
			output.sqlcStepImage = image
		}

		for _, step := range job.Steps {
			uses := step.Uses
			switch {
			case strings.Contains(uses, "ci-resources"):
				output.ciResourcesAction = uses
			}
		}
	}

	// find the actions verison comment added by renovate, if present
	if output.ciResourcesAction != "" {
		scanner := bufio.NewScanner(bytes.NewReader(rawConfig))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, output.ciResourcesAction) {
				chunks := strings.Split(line, "#")
				if len(chunks) > 1 {
					output.ciResourcesActionTag = strings.TrimSpace(chunks[1])
					break
				}
			}
		}

	}

	return output
}

func (s *ResourceSet) populateMissingResources(privateGitDomain string) {
	// these defaults will slowly get out of date, but they will only be applied to first-time ci and Renovate will update them anyway

	if s.ciResourcesAction == "" {
		if privateGitDomain == "" {
			s.ciResourcesAction = "markormesher/ci-resources/setup@v0.5.0"
		} else {
			s.ciResourcesAction = fmt.Sprintf("https://%s/mormesher/ci-resources/setup@v0.5.0", privateGitDomain)
		}
	}

	if s.bufStepImage == "" {
		s.bufStepImage = "docker.io/bufbuild/buf:1.61.0"
	}

	if s.goStepImage == "" {
		s.goStepImage = "docker.io/golang:1.26.5"
	}

	if s.imgStepImage == "" || !strings.Contains(s.imgStepImage, "-immutable") {
		s.imgStepImage = "quay.io/podman/stable:v5.7.1-immutable"
	}

	if s.jsStepImage == "" || strings.Contains(s.jsStepImage, "-slim") {
		s.jsStepImage = "docker.io/node:25.9.0"
	}

	if s.sqlcStepImage == "" {
		s.sqlcStepImage = "docker.io/sqlc/sqlc:1.28.0"
	}

	if s.utilStepImage == "" {
		s.utilStepImage = "docker.io/busybox:1.37.0"
	}
}
