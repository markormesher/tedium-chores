package util

import (
	"regexp"
	"slices"
)

func MatchingStrings(candidates []string, patterns []*regexp.Regexp) []string {
	matches := []string{}
	for _, candidate := range candidates {
		for _, pattern := range patterns {
			if pattern.MatchString(candidate) && !slices.Contains(matches, candidate) {
				matches = append(matches, candidate)
			}
		}
	}
	return matches
}
