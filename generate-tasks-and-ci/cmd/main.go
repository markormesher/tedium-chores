package main

import (
	"flag"
	"os"
	"strings"

	"github.com/markormesher/tedium-chores/generate-tasks-and-ci/internal/log"
)

var l = log.Logger

func main() {
	// read config and validate it
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

	updateTaskfile(projectPath)
	updateCIConfig(projectPath, ciType)
}
