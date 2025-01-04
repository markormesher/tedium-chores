package util

import (
	"os"
	"regexp"
	"slices"
)

func SliceIsSubset[T comparable](outer []T, inner []T) bool {
	for i := range inner {
		if !slices.Contains(outer, inner[i]) {
			return false
		}
	}

	return true
}

func MatchingStrings(candidates []string, patterns []regexp.Regexp) []string {
	matches := make([]string, 0)
	for _, candidate := range candidates {
		for _, pattern := range patterns {
			if pattern.MatchString(candidate) && !slices.Contains(matches, candidate) {
				matches = append(matches, candidate)
			}
		}
	}
	return matches
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
