package util

import "slices"

func SliceIsSubset[T comparable](outer []T, inner []T) bool {
	for i := range inner {
		if !slices.Contains(outer, inner[i]) {
			return false
		}
	}

	return true
}
