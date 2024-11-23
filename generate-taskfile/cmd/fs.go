package main

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

var (
	FIND_FILES = 1
	FIND_DIRS  = 2
)

func find(targets int, patterns []*regexp.Regexp, excludePatterns []*regexp.Regexp) ([]string, error) {
	// TODO: replace with fs.WalkDir

	findFiles := targets&FIND_FILES != 0
	findDirs := targets&FIND_DIRS != 0

	var directoryFrontier Queue[string]
	directoriesVisited := make(map[string]bool)
	matches := make([]string, 0)

	directoryFrontier.Push(projectPath)

	for {
		directory, ok := directoryFrontier.Pop()
		if !ok {
			break
		}

		directoriesVisited[*directory] = true

		entries, err := os.ReadDir(*directory)
		if err != nil {
			return nil, fmt.Errorf("error searching for files or directories: %w", err)
		}

		for _, entry := range entries {
			// full path is needed to walk the project tree, but only the project-relative path is used by the rest of the program
			fullPath := path.Join(*directory, entry.Name())
			relativePath := strings.TrimPrefix(fullPath, projectPath)

			match := false

			for i := range patterns {
				if patterns[i].MatchString(relativePath) {
					match = true
					break
				}
			}

			if match {
				for i := range excludePatterns {
					if excludePatterns[i].MatchString(relativePath) {
						match = false
						break
					}
				}
			}

			if entry.Type().IsDir() {
				if findDirs && match {
					matches = append(matches, fullPath)
				}

				if !directoriesVisited[fullPath] {
					directoryFrontier.Push(fullPath)
				}
			}

			if entry.Type().IsRegular() {
				if findFiles && match {
					matches = append(matches, relativePath)
				}
			}
		}
	}

	return matches, nil
}

func pathToSafeName(path string) string {
	if path == "/" {
		return "root"
	}

	illegalChars := regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)
	multipleDashes := regexp.MustCompile(`\-+`)

	path = illegalChars.ReplaceAllString(path, "-")
	path = multipleDashes.ReplaceAllString(path, "-")
	path = strings.Trim(path, "-")

	return path
}
