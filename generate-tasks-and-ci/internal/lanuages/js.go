package lanuages

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/task"
	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/util"
)

type JSProject struct {
	ParentPath        string
	ProjectPath       string
	RelativePath      string
	PackageManagerCmd string
	Config            PackageJSON
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
			slog.Warn("skipping JS/TS project with unsupported package manager", "packageManager", config.PackageManager)
			continue
		}

		output = append(output, &JSProject{
			ParentPath:        projectPath,
			ProjectPath:       path.Join(projectPath, path.Dir(p)),
			RelativePath:      path.Dir(p),
			PackageManagerCmd: packageManagerCmd,
			Config:            config,
		})
	}

	return output, nil
}

func (p *JSProject) GetProjectPath() string {
	return p.ProjectPath
}

func (p *JSProject) GetRelativePath() string {
	return p.RelativePath
}

func (p *JSProject) AddTasks(taskFile *task.TaskFile) error {
	adders := []TaskAdder{
		p.addCacheKeyTask,
		p.addCacheLoadTask,
		p.addCacheSaveTask,
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

func (p *JSProject) addCacheKeyTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("cachekey-%s-js", util.PathToSafeName(p.RelativePath))

	nonAlphanumeric := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	projectName := nonAlphanumeric.ReplaceAllString(path.Base(p.ParentPath), "")

	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Generates: []string{".task-meta-cache-key"},
		Commands: []task.Command{
			{
				Command: `
if [ ${CI:+1} ]; then
  PROJECT="` + projectName + `"

  DEPS_SHA=$(cat package.json | sha256sum | awk '{ print $1 }')

  if [[ -f pnpm-lock.yaml ]]; then
    LOCK_SHA=$(cat pnpm-lock.yaml | sha256sum | awk '{ print $1 }')
  elif [[ -f yarn.lock ]]; then
    LOCK_SHA=$(cat yarn.lock | sha256sum | awk '{ print $1 }')
  else
    LOCK_SHA="nolock"
  fi

  echo "${PROJECT}-js-v1/${DEPS_SHA}/${LOCK_SHA}" > .task-meta-cache-key
fi
`,
			},
		},
	}

	return nil
}

func (p *JSProject) addCacheLoadTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("cacheload-%s-js", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Dependencies: []string{
			fmt.Sprintf("cachekey-%s-js", util.PathToSafeName(p.RelativePath)),
		},
		Commands: []task.Command{
			{Command: cacheLoadCommand()},
		},
	}

	return nil
}

func (p *JSProject) addCacheSaveTask(taskFile *task.TaskFile) error {
	name := fmt.Sprintf("cachesave-%s-js", util.PathToSafeName(p.RelativePath))

	cachePathCmd := ""
	switch p.PackageManagerCmd {
	case "pnpm":
		cachePathCmd = "pnpm store path"

	case "yarn":
		cachePathCmd = "yarn cache dir"

	default:
		return fmt.Errorf("encountered unsupported package manager '%s' when generating cachesave-js task", p.PackageManagerCmd)
	}

	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Dependencies: []string{
			fmt.Sprintf("cachekey-%s-js", util.PathToSafeName(p.RelativePath)),
		},
		Commands: []task.Command{
			{Command: cachePathCmd + ` > .task-meta-cache-paths`},
			{Command: cacheSaveCommand()},
		},
	}

	return nil
}

func (p *JSProject) addDepsTask(taskFile *task.TaskFile) error {
	cmds := []task.Command{}

	switch p.PackageManagerCmd {

	case "pnpm":
		cmds = append(
			cmds,
			task.Command{Command: "pnpm install --frozen-lockfile"},
			task.Command{Command: "pnpm peers check"},
		)

	case "yarn":
		cmds = append(
			cmds,
			task.Command{Command: "yarn install --immutable"},
		)

	default:
		return fmt.Errorf("encountered unsupported package manager '%s' when generating deps-js task", p.PackageManagerCmd)
	}

	name := fmt.Sprintf("deps-%s-js", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands:  cmds,
	}

	return nil
}

func (p *JSProject) addLintTask(taskFile *task.TaskFile) error {
	if _, ok := p.Config.Scripts["lint"]; !ok {
		return nil
	}

	name := fmt.Sprintf("lint-%s-js", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
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

	name := fmt.Sprintf("lintfix-%s-js", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
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

	name := fmt.Sprintf("test-%s-js", util.PathToSafeName(p.RelativePath))
	taskFile.Tasks[name] = &task.Task{
		Directory: path.Join("{{.ROOT_DIR}}", p.RelativePath),
		Commands: []task.Command{
			{Command: fmt.Sprintf(`%s test`, p.PackageManagerCmd)},
		},
	}

	return nil
}
