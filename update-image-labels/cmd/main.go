package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var jsonHandler = slog.NewJSONHandler(os.Stdout, nil)
var l = slog.New(jsonHandler)

var rootPath = "/tedium/repo"

func main() {
	// default labels - all empty
	labels := map[string]string{
		// labels we will try to set later
		"org.opencontainers.image.url":   "",
		"org.opencontainers.image.title": "",

		// common inherited to remove from base images
		"org.opencontainers.image.vendor":        "",
		"org.opencontainers.image.description":   "",
		"org.opencontainers.image.version":       "",
		"org.opencontainers.image.documentation": "",
	}

	// set labels from env if possible
	repoOwner := os.Getenv("TEDIUM_REPO_OWNER")
	repoName := os.Getenv("TEDIUM_REPO_NAME")
	repoDomain := os.Getenv("TEDIUM_PLATFORM_DOMAIN")
	if repoOwner != "" && repoName != "" && repoDomain != "" {
		labels["org.opencontainers.image.title"] = repoName
		labels["org.opencontainers.image.url"] = fmt.Sprintf("https://%s/%s/%s", repoDomain, repoOwner, repoName)
	}

	// find files to update
	err := fs.WalkDir(os.DirFS(rootPath), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			l.Error("error finding containerfiles", "error", err)
			os.Exit(1)
		}

		if d.Name() == "Containerfile" || d.Name() == "Dockerfile" {
			fullPath := filepath.Join(rootPath, path)
			err := processFile(fullPath, labels)
			if err != nil {
				l.Error("error processing file", "path", fullPath, "error", err)
				os.Exit(1)
			}
		}

		return nil
	})

	if err != nil {
		l.Error("error finding containerfiles", "error", err)
		os.Exit(1)
	}
}

func processFile(path string, labels map[string]string) error {
	l.Info("processing file", "file", path)

	// open the file and read it into lines
	containerFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		err := containerFile.Close()
		if err != nil {
			l.Error("error closing file", "error", err)
		}
	}()

	lines := []string{}
	scanner := bufio.NewScanner(containerFile)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// process the actual lines
	lines = processLines(lines, labels)
	output := strings.Join(lines, "\n")
	err = os.WriteFile(path, []byte(output), os.ModePerm)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func processLines(lines []string, labels map[string]string) []string {
	// separate final-stage lines from any earlier stages
	finalStageLines := []string{}
	nonFinalStageLines := []string{}
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "from ") {
			nonFinalStageLines = append(nonFinalStageLines, finalStageLines...)
			finalStageLines = []string{line}
		} else {
			finalStageLines = append(finalStageLines, line)
		}
	}

	// separate labels in the final stage
	labelLines := []string{}
	nonLabelLines := []string{}
	for _, line := range finalStageLines {
		if strings.HasPrefix(strings.ToLower(line), "label ") {
			labelLines = append(labelLines, line)
		} else {
			nonLabelLines = append(nonLabelLines, line)
		}
	}

	// capitalise "label" statements
	for i, line := range labelLines {
		labelLines[i] = "LABEL" + line[5:]
	}

	// insert or update labels
	for key, value := range labels {
		idx := slices.IndexFunc(labelLines, func(line string) bool {
			return strings.HasPrefix(line, fmt.Sprintf("LABEL %s", key))
		})
		if idx < 0 {
			labelLines = append(labelLines, fmt.Sprintf("LABEL %s=%q", key, value))
		} else {
			labelLines[idx] = fmt.Sprintf("LABEL %s=%q", key, value)
		}
	}

	slices.Sort(labelLines)

	// reassemble final stage
	nonLabelLines = append(nonLabelLines, "")
	finalStageLines = append(nonLabelLines, labelLines...)

	// reassemble file
	lines = append(nonFinalStageLines, finalStageLines...)
	lines = append(lines, "")
	lines = removeExtraBlanks(lines)

	return lines
}

func removeExtraBlanks(lines []string) []string {
	newLines := []string{}

	// condense consecuitive blank lines
	for _, line := range lines {
		if len(newLines) > 0 && newLines[len(newLines)-1] == "" && line == "" {
			continue
		}
		newLines = append(newLines, line)
	}

	return newLines
}
