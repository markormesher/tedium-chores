package util

import (
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
