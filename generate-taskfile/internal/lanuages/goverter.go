package lanuages

import (
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/markormesher/tedium-chores/generate-taskfile/internal/task"
	"github.com/markormesher/tedium-chores/generate-taskfile/internal/util"
)

type GoverterProject struct {
	ProjectRelativePath string
	GoverterFilePaths   []string
}

func FindGoverterProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	goModPaths, err := util.Find(
		projectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)go\.mod`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error searching for Goverter projects: %w", err)
	}

	for _, p := range goModPaths {
		goModPath := path.Join(projectPath, p)
		match, err := util.FileContains(goModPath, "tool github.com/jmattheis/goverter/cmd/goverter")
		if err != nil {
			return nil, fmt.Errorf("error searching for Goverter projects: %w", err)
		}

		if match {
			goverterFiles, err := findGoverterProjectFiles(path.Dir(goModPath))
			if err != nil {
				return nil, fmt.Errorf("error searching for Goverter projects: %w", err)
			}

			if len(goverterFiles) > 0 {
				output = append(output, &GoverterProject{
					ProjectRelativePath: path.Dir(p),
					GoverterFilePaths:   goverterFiles,
				})
			}
		}
	}

	return output, nil
}

func findGoverterProjectFiles(projectPath string) ([]string, error) {
	goFilePaths, err := util.Find(
		projectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/).*\.go$`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error searching for Go files within Goverter project: %w", err)
	}

	out := map[string]struct{}{}
	for _, filePath := range goFilePaths {
		match, err := util.FileContains(path.Join(projectPath, filePath), "// goverter:converter")
		if err != nil {
			return nil, fmt.Errorf("error checking for Goverter file: %w", err)
		}
		if match {
			outputPath := "./" + path.Dir(filePath)
			out[outputPath] = struct{}{}
		}
	}

	return slices.Collect(maps.Keys(out)), nil
}

func (p *GoverterProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
		p.addGenTask,
	}

	for _, f := range adders {
		err := f(taskFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *GoverterProject) addGenTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("gen-goverter-%s", util.PathToSafeName(p.ProjectRelativePath))
	safePaths := make([]string, len(p.GoverterFilePaths))
	for i, path := range p.GoverterFilePaths {
		safePaths[i] = strconv.Quote(path)
	}

	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{
				Command: fmt.Sprintf("go tool github.com/jmattheis/goverter/cmd/goverter gen %s", strings.Join(safePaths, " ")),
			},
		},
	}

	return nil
}
