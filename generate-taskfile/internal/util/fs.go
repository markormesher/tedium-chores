package util

import (
	"bufio"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

var (
	FIND_FILES = 1
	FIND_DIRS  = 2
)

func Find(projectPath string, targets int, patterns []*regexp.Regexp, excludePatterns []*regexp.Regexp) ([]string, error) {
	findFiles := targets&FIND_FILES != 0
	findDirs := targets&FIND_DIRS != 0
	matches := make([]string, 0)

	projectFs := os.DirFS(projectPath)
	fs.WalkDir(projectFs, ".", func(path string, d fs.DirEntry, err error) error {
		relativePath := strings.TrimPrefix(path, projectPath)
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

		if match && findDirs && d.IsDir() {
			matches = append(matches, path)
		}

		if match && findFiles && !d.IsDir() {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, nil
}

func PathToSafeName(path string) string {
	if path == "." {
		return "root"
	}

	illegalChars := regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)
	multipleDashes := regexp.MustCompile(`\-+`)

	path = illegalChars.ReplaceAllString(path, "-")
	path = multipleDashes.ReplaceAllString(path, "-")
	path = strings.Trim(path, "-")

	return path
}

func FileContainsLine(path string, line string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), line) {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}
