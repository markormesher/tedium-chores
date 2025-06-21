package main

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/ciconfig"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
	"gopkg.in/yaml.v3"
)

const cacheVersion = 5

// ImageSet is a utility type to store the container image references used for various steps.
type ImageSet struct {
	bufStepImage       string
	fetchTaskStepImage string
	gitStepImage       string
	goStepImage        string
	imgStepImage       string
	jsStepImage        string
	sqlcStepImage      string
	utilStepImage      string
}

func updateCIConfig(projectPath string, ciType string) {
	projectPath = strings.TrimRight(projectPath, "/")

	projectPathExists, err := util.DirExists(projectPath)
	if err != nil {
		l.Error("error checking whether project path exists", "error", err)
		os.Exit(1)
	}

	if !projectPathExists {
		l.Error("project path does not exist", "path", projectPath)
		os.Exit(1)
	}

	if ciType == "auto" {
		if droneFileExists, _ := util.FileExists(path.Join(projectPath, ciconfig.DroneFilePath)); droneFileExists {
			ciType = "drone"
		} else if circleFileExists, _ := util.FileExists(path.Join(projectPath, ciconfig.CircleCiFilePath)); circleFileExists {
			ciType = "circle"
		} else {
			l.Error("unable to determine CI type automatically")
			os.Exit(1)
		}
	}

	if ciType != "drone" && ciType != "circle" {
		l.Error("Unsupported CI type", "ciType", ciType)
		os.Exit(1)
	}

	// define the output path based on the CI type
	outputPath := ""
	switch ciType {
	case "drone":
		outputPath = path.Join(projectPath, ciconfig.DroneFilePath)
	case "circle":
		outputPath = path.Join(projectPath, ciconfig.CircleCiFilePath)
	}

	// read taskfile
	taskfilePath := path.Join(projectPath, "taskfile.yml")
	taskfile, err := task.LoadTaskFile(taskfilePath)
	if err != nil {
		l.Error("error loading Taskfile", "error", err)
		os.Exit(1)
	}
	if taskfile == nil {
		l.Warn("no taskfile in this repo - skipping")
		os.Exit(0)
	}

	taskNames := make([]string, 0)
	for name, task := range taskfile.Tasks {
		if !task.Internal {
			taskNames = append(taskNames, name)
		}
	}

	// extract image versions from existing CI files if possible
	imageSet := ImageSet{}

	switch ciType {
	case "drone":
		droneConfig, err := ciconfig.LoadDroneConfigIfPresent(outputPath)
		if err != nil {
			l.Warn("Error reading existing Drone config - continuing without it", "error", err)
		}
		if droneConfig != nil {
			imageSet = extractImagesFromDroneConfig(*droneConfig)
		}

	case "circle":
		circleConfig, err := ciconfig.LoadCircleConfigIfPresent(outputPath)
		if err != nil {
			l.Warn("Error reading existing Circle config - continuing without it", "error", err)
		}
		if circleConfig != nil {
			imageSet = extractImagesFromCircleConfig(*circleConfig)
		}
	}

	imageSet.populateMissingImages(ciType)

	// generate CI steps based on tasks
	steps := make([]*ciconfig.GenericCiStep, 0)

	if ciType == "circle" {
		steps = append(steps, &ciconfig.GenericCiStep{
			Name:           "checkout",
			Image:          imageSet.utilStepImage,
			IsCheckoutStep: true,
			WorkspacePersistPaths: []string{
				".",
			},
		})
	}

	steps = append(steps, &ciconfig.GenericCiStep{
		Name:  "fetch-task",
		Image: imageSet.fetchTaskStepImage,
		Commands: []string{
			`cp /task .`,
		},
		WorkspacePersistPaths: []string{
			"./task",
		},
	})

	steps = append(steps, &ciconfig.GenericCiStep{
		Name:        "ci-all",
		Image:       imageSet.utilStepImage,
		NoWorkspace: true,
		Commands: []string{
			`echo "Done"`,
		},
		Dependencies: []regexp.Regexp{
			*regexp.MustCompile(`deps\-.*`),
			*regexp.MustCompile(`lint\-.*`),
			*regexp.MustCompile(`test\-.*`),
			*regexp.MustCompile(`img.*`),
		},
	})

	if slices.Contains(taskNames, "cachekey") {
		steps = append(steps, &ciconfig.GenericCiStep{
			Name:  "cachekey",
			Image: imageSet.utilStepImage,
			Commands: []string{
				`./task cachekey`,
			},
			WorkspacePersistPaths: []string{
				"./.task-meta-cachekey*",
			},
			Dependencies: []regexp.Regexp{
				*regexp.MustCompile(`checkout`),
				*regexp.MustCompile(`fetch\-task`),
			},
		})
	}

	if slices.Contains(taskNames, "deps-go") {
		steps = append(steps, &ciconfig.GenericCiStep{
			Name:  "deps-go",
			Image: imageSet.goStepImage,
			CacheRestoreKeys: []string{
				fmt.Sprintf(`deps-go-v%d-{{ checksum ".task-meta-cachekey-go" }}`, cacheVersion),
				fmt.Sprintf(`deps-go-v%d-`, cacheVersion),
			},
			Commands: []string{
				`export GOPATH=/.go`,
				`export GOCACHE=/.gocache`,
				`./task deps-go`,
			},
			CacheSaveKey: fmt.Sprintf(`deps-go-v%d-{{ checksum ".task-meta-cachekey-go" }}`, cacheVersion),
			CacheSavePaths: []string{
				"/.go",
				"/.gocache",
			},
			Dependencies: []regexp.Regexp{
				*regexp.MustCompile(`checkout`),
				*regexp.MustCompile(`fetch\-task`),
				*regexp.MustCompile(`cachekey`),
			},
		})
	}

	if slices.Contains(taskNames, "deps-js") {
		// circle's save-cache step doesn't support globs, so we have to build this path ourselves
		cachePaths := []string{
			"/usr/local/lib/node_modules",
		}
		for _, t := range taskNames {
			if strings.HasPrefix(t, "deps-js-") {
				cachePaths = append(cachePaths, strings.ReplaceAll(taskfile.Tasks[t].Directory, "{{.ROOT_DIR}}/", "")+"/node_modules")
			}
		}

		steps = append(steps, &ciconfig.GenericCiStep{
			Name:  "deps-js",
			Image: imageSet.jsStepImage,
			CacheRestoreKeys: []string{
				fmt.Sprintf(`deps-js-v%d-{{ checksum ".task-meta-cachekey-js" }}`, cacheVersion),
				fmt.Sprintf(`deps-js-v%d-`, cacheVersion),
			},
			Commands: []string{
				`corepack enable`,
				`./task deps-js`,
			},
			CacheSaveKey:   fmt.Sprintf(`deps-js-v%d-{{ checksum ".task-meta-cachekey-js" }}`, cacheVersion),
			CacheSavePaths: cachePaths,
			Dependencies: []regexp.Regexp{
				*regexp.MustCompile(`checkout`),
				*regexp.MustCompile(`fetch\-task`),
				*regexp.MustCompile(`cachekey`),
			},
		})
	}

	lintOrTestTaskRegex := regexp.MustCompile(`(lint|test)\-[a-z]+$`)
	for _, name := range taskNames {
		if lintOrTestTaskRegex.MatchString(name) {
			lang := strings.Split(name, "-")[1]
			image, err := getImageForLanguageTask(imageSet, name)
			if err != nil {
				l.Error("unable to determine image for step", "error", err)
				os.Exit(1)
			}

			commands := make([]string, 0)

			// add runtime dependencies
			runtimePackages := os.Getenv(fmt.Sprintf("%s_RUNTIME_PACKAGES", strings.ToUpper(lang)))
			if runtimePackages != "" {
				commands = append(commands, fmt.Sprintf(`apt update && apt install -y --no-install-recommends %s`, runtimePackages))
			}

			// add runtime environment
			runtimeEnvPrefix := fmt.Sprintf("%s_RUNTIME_ENV_", strings.ToUpper(lang))
			for _, env := range os.Environ() {
				envPair := strings.SplitN(env, "=", 2)
				if strings.HasPrefix(envPair[0], runtimeEnvPrefix) {
					commands = append(commands, fmt.Sprintf(`export %s="%s"`, strings.TrimPrefix(envPair[0], runtimeEnvPrefix), envPair[1]))
				}
			}

			// add language-specific setup steps
			switch lang {
			case "go":
				commands = append(commands, `export GOPATH=/.go`)
				commands = append(commands, `export GOCACHE=/.gocache`)
			case "js":
				commands = append(commands, `corepack enable`)
			}

			commands = append(commands, fmt.Sprintf("./task %s", name))

			steps = append(steps, &ciconfig.GenericCiStep{
				Name:     name,
				Image:    image,
				Commands: commands,
				CacheRestoreKeys: []string{
					fmt.Sprintf(`deps-%s-v%d-{{ checksum ".task-meta-cachekey-%s" }}`, lang, cacheVersion, lang),
					fmt.Sprintf("deps-%s-v%d-", lang, cacheVersion),
				},
				Dependencies: []regexp.Regexp{
					*regexp.MustCompile(`checkout`),
					*regexp.MustCompile(`fetch\-task`),
					*regexp.MustCompile(fmt.Sprintf(`deps\-%s`, lang)),
				},
			})
		}
	}

	if slices.Contains(taskNames, "imgrefs") {
		// refs
		commands := make([]string, 0)
		if ciType == "drone" {
			commands = append(commands, `git fetch --tags`)
		}
		commands = append(commands, `./task imgrefs`)

		steps = append(steps, &ciconfig.GenericCiStep{
			Name:     "imgrefs",
			Image:    imageSet.gitStepImage,
			Commands: commands,
			WorkspacePersistPaths: []string{
				"./.task-meta-imgrefs",
				"./**/.task-meta-imgrefs",
			},
			Dependencies: []regexp.Regexp{
				*regexp.MustCompile(`checkout`),
				*regexp.MustCompile(`fetch\-task`),
			},
		})

		// build + push
		commands = make([]string, 0)
		if ciType == "circle" {
			commands = append(commands, `echo "${GHCR_PUBLISH_TOKEN}" | docker login ghcr.io -u markormesher --password-stdin`)
		}
		commands = append(commands, `./task imgbuild`)
		commands = append(commands, `./task imgpush`)

		step := ciconfig.GenericCiStep{
			Name:     "imgbuild-imgpush",
			Image:    imageSet.imgStepImage,
			Commands: commands,
			Dependencies: []regexp.Regexp{
				*regexp.MustCompile(`checkout`),
				*regexp.MustCompile(`fetch\-task`),
				*regexp.MustCompile(`lint\-.*`),
				*regexp.MustCompile(`test\-.*`),
				*regexp.MustCompile(`imgrefs`),
			},
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
		matchedSteps := util.MatchingStrings(stepNames, step.Dependencies)
		slices.Sort(matchedSteps)
		step.ResolvedDependencies = matchedSteps
	}

	// sort steps by name and then by topology to make the output deterministic and readable
	sort.Slice(steps, func(a, b int) bool {
		return cmp.Less(steps[a].Name, steps[b].Name)
	})

	sortedSteps := make([]*ciconfig.GenericCiStep, 0)
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
	var output any
	switch ciType {
	case "drone":
		output = ciconfig.GenerateDroneConfig(sortedSteps)
	case "circle":
		output = ciconfig.GenerateCircleConfig(sortedSteps)
	}

	var outputBuffer bytes.Buffer
	encoder := yaml.NewEncoder(&outputBuffer)
	encoder.SetIndent(2)
	err = encoder.Encode(output)
	if err != nil {
		l.Error("Couldn't marshall output")
		os.Exit(1)
	}

	handleWriteError := func(err error) {
		if err != nil {
			l.Error("Error writing to CI config", "error", err)
			os.Exit(1)
		}
	}

	err = os.MkdirAll(path.Dir(outputPath), 0755)
	handleWriteError(err)

	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	handleWriteError(err)
	defer func() {
		_ = outputFile.Close()
	}()

	_, err = outputFile.WriteString("# This file is maintained by Tedium - manual edits will be overwritten!\n\n")
	handleWriteError(err)

	_, err = outputFile.Write(outputBuffer.Bytes())
	handleWriteError(err)
}

func getImageForLanguageTask(imageSet ImageSet, taskName string) (string, error) {
	lang := strings.Split(taskName, "-")[1]
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

func extractImagesFromDroneConfig(config ciconfig.DroneConfig) ImageSet {
	output := ImageSet{}

	for _, step := range config.Steps {
		image := step.Image
		switch {
		case strings.Contains(image, "bufbuild"):
			output.bufStepImage = image
		case strings.Contains(image, "busybox"):
			output.utilStepImage = image
		case strings.Contains(image, "git"):
			output.gitStepImage = image
		case strings.Contains(image, "golang"):
			output.goStepImage = image
		case strings.Contains(image, "node"):
			output.jsStepImage = image
		case strings.Contains(image, "podman"):
			output.imgStepImage = image
		case strings.Contains(image, "sqlc"):
			output.sqlcStepImage = image
		case strings.Contains(image, "task-fetcher"):
			output.fetchTaskStepImage = image
		}
	}

	return output
}

func extractImagesFromCircleConfig(config ciconfig.CircleConfig) ImageSet {
	output := ImageSet{}

	for _, job := range config.Jobs {
		if len(job.Docker) < 1 {
			continue
		}

		image := job.Docker[0].Image
		switch {
		case strings.Contains(image, "bufbuild"):
			output.bufStepImage = image
		case strings.Contains(image, "cimg/base"):
			output.utilStepImage = image
			output.imgStepImage = image
		case strings.Contains(image, "git"):
			output.gitStepImage = image
		case strings.Contains(image, "golang"):
			output.goStepImage = image
		case strings.Contains(image, "node"):
			output.jsStepImage = image
		case strings.Contains(image, "sqlc"):
			output.sqlcStepImage = image
		case strings.Contains(image, "task-fetcher"):
			output.fetchTaskStepImage = image
		}
	}

	return output
}

func (s *ImageSet) populateMissingImages(ciType string) {
	// these defaults will slowly get out of date, but they will only be applied to first-time ciconfig and Renovate will update them anyway

	if s.bufStepImage == "" {
		s.bufStepImage = "docker.io/bufbuild/buf:1.48.0"
	}

	if s.fetchTaskStepImage == "" {
		s.fetchTaskStepImage = "ghcr.io/markormesher/task-fetcher:v0.4.1"
	}

	if s.gitStepImage == "" {
		s.gitStepImage = "docker.io/alpine/git:v2.47.1"
	}

	if s.goStepImage == "" {
		s.goStepImage = "docker.io/golang:1.23.4"
	}

	if s.imgStepImage == "" {
		if ciType == "circle" {
			s.imgStepImage = "cimg/base:2024.12"
		} else {
			s.imgStepImage = "quay.io/podman/stable:v5.3.1"
		}
	}

	if s.jsStepImage == "" {
		s.jsStepImage = "docker.io/node:23.5.0-slim"
	}

	if s.sqlcStepImage == "" {
		s.sqlcStepImage = "docker.io/sqlc/sqlc:1.28.0"
	}

	if s.utilStepImage == "" {
		if ciType == "circle" {
			s.utilStepImage = "cimg/base:2024.12"
		} else {
			s.utilStepImage = "docker.io/busybox:1.37.0"
		}
	}
}
