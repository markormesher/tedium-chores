package lanuages

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/markormesher/tedium-chores/generate-taskfile/internal/logging"
	"github.com/markormesher/tedium-chores/generate-taskfile/internal/task"
	"github.com/markormesher/tedium-chores/generate-taskfile/internal/util"
)

var l = logging.Logger

type JSProject struct {
	ProjectRelativePath string
	PackageManagerCmd   string
	Config              PackageJSON
}

type PackageJSON struct {
	// partial representation
	Scripts        map[string]string `json:"scripts"`
	PackageManager string            `json:"packageManager"`
}

func FindJSProjects(projectPath string) ([]Project, error) {
	output := []Project{}

	packageJSONPaths, err := util.Find(
		projectPath,
		util.FIND_FILES,
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)package\.json`),
		},
		[]*regexp.Regexp{
			regexp.MustCompile(`(^|/)\.git/`),
			regexp.MustCompile(`(^|/)node_modules/`),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error searching for JS/TS projects: %w", err)
	}

	for _, p := range packageJSONPaths {
		contents, err := os.ReadFile(path.Join(projectPath, p))
		if err != nil {
			return nil, fmt.Errorf("error reading package.json: %w", err)
		}

		var config PackageJSON
		err = json.Unmarshal(contents, &config)
		if err != nil {
			return nil, fmt.Errorf("error parsing package.json: %w", err)
		}

		packageManagerCmd := ""
		switch {
		case strings.HasPrefix(config.PackageManager, "pnpm"):
			packageManagerCmd = "pnpm"

		case strings.HasPrefix(config.PackageManager, "yarn"):
			packageManagerCmd = "yarn"

		// supporting a new package manager? don't forget to update other switch statements

		default:
			l.Warn("skipping JS/TS project with unsupported package manager", "packageManager", config.PackageManager)
			continue
		}

		output = append(output, &JSProject{
			ProjectRelativePath: path.Dir(p),
			PackageManagerCmd:   packageManagerCmd,
			Config:              config,
		})
	}

	return output, nil
}

func (p *JSProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
		p.addDepsTask,
		p.addLintTask,
		p.addLintFixTask,
		p.addTestTask,
	}

	for _, f := range adders {
		err := f(taskFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *JSProject) addDepsTask(taskFile *task.TaskFile) error {
	cmd := ""

	switch p.PackageManagerCmd {

	case "pnpm":
		cmd = "pnpm install --frozen-lockfile"

	case "yarn":
		cmd = "yarn install --immutable"

	default:
		return fmt.Errorf("encountered unsupported package manager '%s' when generating deps-js task", p.PackageManagerCmd)
	}

	name := fmt.Sprintf("deps-js-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: cmd},
		},
	}

	return nil
}

func (p *JSProject) addLintTask(taskFile *task.TaskFile) error {
	if _, ok := p.Config.Scripts["lint"]; !ok {
		return nil
	}

	name := fmt.Sprintf("lint-js-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: fmt.Sprintf(`%s lint`, p.PackageManagerCmd)},
		},
	}

	return nil
}

func (p *JSProject) addLintFixTask(taskFile *task.TaskFile) error {
	if _, ok := p.Config.Scripts["lintfix"]; !ok {
		return nil
	}

	name := fmt.Sprintf("lintfix-js-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: fmt.Sprintf(`%s lintfix`, p.PackageManagerCmd)},
		},
	}

	return nil
}

func (p *JSProject) addTestTask(taskFile *task.TaskFile) error {
	if _, ok := p.Config.Scripts["test"]; !ok {
		return nil
	}

	name := fmt.Sprintf("test-js-%s", util.PathToSafeName(p.ProjectRelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.ProjectRelativePath),
		Commands: []task.Command{
			{Command: fmt.Sprintf(`%s test`, p.PackageManagerCmd)},
		},
	}

	return nil
}
