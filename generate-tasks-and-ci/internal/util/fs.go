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
	err := fs.WalkDir(projectFs, ".", func(path string, d fs.DirEntry, err error) error {
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

	if err != nil {
		return nil, err
	}

	return matches, nil
}

func PathToSafeName(path string) string {
	if path == "." {
		return "root"
	}

	illegalChars := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	path = illegalChars.ReplaceAllString(path, "")

	return path
}

func FileContains(path string, line string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = f.Close()
	}()

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

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == os.ErrNotExist {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func DirExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == os.ErrNotExist {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if !stat.IsDir() {
		return false, nil
	}

	return true, nil
}
