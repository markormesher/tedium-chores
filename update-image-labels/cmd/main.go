package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
)

var jsonHandler = slog.NewJSONHandler(os.Stdout, nil)
var l = slog.New(jsonHandler)

var labels = map[string]string{
	// labels we will try to set later
	"org.opencontainers.image.url":   "",
	"org.opencontainers.image.title": "",

	// remove common labels inherited from base images
	"org.opencontainers.image.vendor":        "",
	"org.opencontainers.image.description":   "",
	"org.opencontainers.image.version":       "",
	"org.opencontainers.image.documentation": "",
}

func main() {
	// set labels from env if possible
	repoOwner := os.Getenv("TEDIUM_REPO_OWNER")
	repoName := os.Getenv("TEDIUM_REPO_NAME")
	repoDomain := os.Getenv("TEDIUM_PLATFORM_DOMAIN")
	if repoOwner != "" && repoName != "" && repoDomain != "" {
		labels["org.opencontainers.image.title"] = repoName
		labels["org.opencontainers.image.url"] = fmt.Sprintf("https://%s/%s/%s", repoDomain, repoOwner, repoName)
	}

	err := fs.WalkDir(os.DirFS("/tedium/repo"), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			l.Error("error finding containerfiles", "error", err)
			os.Exit(1)
		}

		if d.Name() == "Containerfile" || d.Name() == "Dockerfile" {
			err := processFile(path)
			l.Error("error processing file", "path", path, "error", err)
			os.Exit(1)
		}

		return nil
	})

	if err != nil {
		l.Error("error finding containerfiles", "error", err)
		os.Exit(1)
	}

}

func processFile(path string) error {
	// open the file and read it into lines
	containerFile, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
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

	// apply operations
	lines = moveLabelsToEnd(lines)

	for key, value := range labels {
		lines = setLabel(lines, key, value)
	}

	lines = removeExtraBlanks(lines)

	// re-write the file
	output := strings.Join(lines, "\n")
	err = os.WriteFile(path, []byte(output), os.ModePerm)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func moveLabelsToEnd(lines []string) []string {
	// if the last line isn't already a label, add a blank line
	if !strings.HasPrefix(strings.ToLower(lines[len(lines)-1]), "label") {
		lines = append(lines, "")
	}

	// track which line numbers contain labels that belong to the final layer
	finalLayerLabelLines := []int{}
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "label") {
			finalLayerLabelLines = append(finalLayerLabelLines, i)
		} else if strings.HasPrefix(strings.ToLower(line), "from") {
			finalLayerLabelLines = []int{}
		}
	}

	// move all the labels to the end (if they were already, this is a no-op)
	linesMoved := 0
	for _, idx := range finalLayerLabelLines {
		moveIdx := idx - linesMoved
		movedLine := lines[moveIdx]
		lines = append(lines[0:moveIdx], lines[moveIdx+1:]...)
		lines = append(lines, movedLine)
		linesMoved++
	}

	return lines
}

func setLabel(lines []string, key string, value string) []string {
	// we already have this label just update the value
	for i, line := range lines {
		if strings.HasPrefix(line, "LABEL") {
			if strings.Contains(line, key) {
				lines[i] = fmt.Sprintf("LABEL %s=%q", key, value)
				return lines
			}
		}
	}

	// we didn't update the value - so insert it
	if !strings.HasPrefix(strings.ToLower(lines[len(lines)-1]), "label") {
		lines = append(lines, "")
	}
	lines = append(lines, fmt.Sprintf("LABEL %s=%q", key, value))

	return lines
}

func removeExtraBlanks(lines []string) []string {
	linesToTrim := []int{}
	lastLineWasBlank := false

	for i, line := range lines {
		if lastLineWasBlank && line == "" {
			linesToTrim = append(linesToTrim, i)
		}

		lastLineWasBlank = line == ""
	}

	trimmedLines := 0
	for _, idx := range linesToTrim {
		trimIdx := idx - trimmedLines
		lines = append(lines[0:trimIdx], lines[trimIdx+1:]...)
		trimmedLines++
	}

	return lines
}
