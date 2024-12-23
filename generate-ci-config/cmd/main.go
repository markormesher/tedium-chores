package main

import (
	"bytes"
	"cmp"
	"flag"
	"log/slog"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/markormesher/tedium-chores/generate-ci-file/internal/schema"
	"github.com/markormesher/tedium-chores/generate-ci-file/internal/util"
	"gopkg.in/yaml.v3"
)

var jsonHandler = slog.NewJSONHandler(os.Stdout, nil)
var l = slog.New(jsonHandler)

func main() {
	// read and validate config
	var projectPath string
	var ciType string
	flag.StringVar(&projectPath, "project", "/tedium/repo", "Project path to target")
	flag.StringVar(&ciType, "ci-type", "auto", "CI type ('drone' or 'circle'")
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

	if ciType == "auto" {
		// TODO: ??
	}

	if ciType != "drone" && ciType != "circle" {
		l.Error("Unsupported CI type", "ciType", ciType)
		os.Exit(1)
	}

	// read and parse task file
	taskfilePath := path.Join(projectPath, "taskfile.yml")
	stat, err = os.Stat(taskfilePath)
	if err != nil {
		l.Error("Error stating Taskfile", "error", err)
		os.Exit(1)
	}

	taskfileContents, err := os.ReadFile(taskfilePath)
	if err != nil {
		l.Error("Error reading Taskfile", "error", err)
		os.Exit(1)
	}

	var taskfile schema.TaskFile
	decoder := yaml.NewDecoder(bytes.NewReader(taskfileContents))
	decoder.KnownFields(false)
	err = decoder.Decode(&taskfile)

	taskNames := make([]string, 0)
	for name, task := range taskfile.Tasks {
		if !task.Internal {
			taskNames = append(taskNames, name)
		}
	}

	// TODO: read and parse incumbent task file if possible

	// determine images to use in steps below (TODO: use existing versions)
	fetchTaskStepImage := "ghcr.io/markormesher/task-fetcher:v0.4.1"
	goStepImage := "docker.io/golang:1.23.4"
	bufStepImage := "docker.io/bufbuild/buf:1.48.0"
	gitStepImage := "docker.io/alpine/git:v2.47.1"
	utilStepImage := ""
	imgStepImage := ""

	switch ciType {
	case "drone":
		utilStepImage = "docker.io/busybox:1.37.0"
		imgStepImage = "quay.io/podman/stable:v5.3.1"

	case "circle":
		utilStepImage = "cimg/base:2024.12"
		imgStepImage = utilStepImage

	}

	// generate CI steps based on tasks
	steps := make([]*schema.GenericCiStep, 0)

	// needed in Circle only: checkout
	if ciType == "circle" {
		steps = append(steps, &schema.GenericCiStep{
			Name:           "checkout",
			Image:          utilStepImage,
			IsCheckoutStep: true,
		})
	}

	// always needed: fetch the task binary
	steps = append(steps, &schema.GenericCiStep{
		Name:  "fetch-task",
		Image: fetchTaskStepImage,
		Commands: []string{
			`cp /task .`,
		},
		Dependencies: []regexp.Regexp{
			*regexp.MustCompile(`checkout`),
		},
	})

	// always needed: final "marker" step for branch protection rules
	steps = append(steps, &schema.GenericCiStep{
		Name:  "ci-all",
		Image: utilStepImage,
		Commands: []string{
			`echo "Done"`,
		},
		Dependencies: []regexp.Regexp{
			*regexp.MustCompile(`checkout`),
			*regexp.MustCompile(`fetch\-task`),
			*regexp.MustCompile(`lint\-.*`),
			*regexp.MustCompile(`test\-.*`),
			*regexp.MustCompile(`img.*`),
		},
		SkipPersist: true,
	})

	// one lint or test step per 2-layer task
	lintOrTestTaskRegex := regexp.MustCompile(`(lint|test)\-[a-z]+$`)
	for _, name := range taskNames {
		if lintOrTestTaskRegex.MatchString(name) {
			lang := strings.Split(name, "-")[1]
			image := ""
			switch lang {
			case "buf":
				image = bufStepImage
			case "go":
				image = goStepImage
			default:
				l.Error("Unsupported language for lint/test task", "language", lang)
			}

			steps = append(steps, &schema.GenericCiStep{
				Name:  name,
				Image: image,
				Commands: []string{
					`./task ` + name,
				},
				Dependencies: []regexp.Regexp{
					*regexp.MustCompile(`checkout`),
					*regexp.MustCompile(`fetch\-task`),
				},
				SkipPersist: true,
			})
		}
	}

	// img steps
	if slices.Contains(taskNames, "imgrefs") {
		// refs
		commands := make([]string, 0)
		if ciType == "drone" {
			commands = append(commands, `git fetch --tags`)
		}
		commands = append(commands, `./task imgrefs`)

		steps = append(steps, &schema.GenericCiStep{
			Name:     "imgrefs",
			Image:    gitStepImage,
			Commands: commands,
			Dependencies: []regexp.Regexp{
				*regexp.MustCompile(`checkout`),
				*regexp.MustCompile(`fetch\-task`),
				*regexp.MustCompile(`lint\-.*`),
				*regexp.MustCompile(`test\-.*`),
			},
		})

		// build + push
		step := schema.GenericCiStep{
			Name:  "imgbuild-imgpush",
			Image: imgStepImage,
			Commands: []string{
				`./task imgbuild`,
				`./task imgpush`,
			},
			Dependencies: []regexp.Regexp{
				*regexp.MustCompile(`checkout`),
				*regexp.MustCompile(`fetch\-task`),
				*regexp.MustCompile(`lint\-.*`),
				*regexp.MustCompile(`test\-.*`),
				*regexp.MustCompile(`imgrefs`),
			},
			SkipPersist: true,
			NeedsDocker: true,
		}

		if ciType == "drone" {
			step.Environment = map[string]string{
				"CONTAINER_HOST": "tcp://podman.podman.svc.cluster.local:8000",
			}
		}

		steps = append(steps, &step)
	}

	// resolve step names
	stepNames := make([]string, 0)
	for _, step := range steps {
		stepNames = append(stepNames, step.Name)
	}

	for _, step := range steps {
		matchedSteps := make([]string, 0)
		for _, candidateStepName := range stepNames {
			for _, dependencyRegex := range step.Dependencies {
				if dependencyRegex.MatchString(candidateStepName) && !slices.Contains(matchedSteps, candidateStepName) {
					matchedSteps = append(matchedSteps, candidateStepName)
				}
			}
		}
		slices.Sort(matchedSteps)
		step.ResolvedDependencies = matchedSteps
	}

	// sort steps by name and then dependency to make the output deterministic and readable
	sort.Slice(steps, func(a, b int) bool {
		return cmp.Less(steps[a].Name, steps[b].Name)
	})

	sortedSteps := make([]*schema.GenericCiStep, 0)
	dependenciesMet := make([]string, 0)
	for len(sortedSteps) < len(steps) {
		sizeBefore := len(sortedSteps)

		for i, step := range steps {
			if step == nil {
				continue
			}

			if util.SliceIsSubset(dependenciesMet, step.ResolvedDependencies) {
				dependenciesMet = append(dependenciesMet, step.Name)
				sortedSteps = append(sortedSteps, step)
				steps[i] = nil
			}
		}

		if sizeBefore == len(sortedSteps) {
			l.Error("Detected a loop in step dependencies")
			os.Exit(1)
		}
	}

	// write out the actual config
	var outputBuffer bytes.Buffer
	outputPath := ""

	switch ciType {
	case "drone":
		outputPath = path.Join(projectPath, ".drone.yml")
		output := schema.GenerateDroneConfig(sortedSteps)
		encoder := yaml.NewEncoder(&outputBuffer)
		encoder.SetIndent(2)
		err = encoder.Encode(output)
		if err != nil {
			l.Error("Couldn't marshall output")
			os.Exit(1)
		}

	case "circle":
		outputPath = path.Join(projectPath, ".circleci/config.yml")
		output := schema.GenerateCircleConfig(sortedSteps)
		encoder := yaml.NewEncoder(&outputBuffer)
		encoder.SetIndent(2)
		err = encoder.Encode(output)
		if err != nil {
			l.Error("Couldn't marshall output")
			os.Exit(1)
		}
	}

	err = os.WriteFile(outputPath, outputBuffer.Bytes(), 0644)
	if err != nil {
		l.Error("Error writing to CI config", "error", err)
		os.Exit(1)
	}
}
