package kit

import (
	"cmp"
	"slices"
	"sort"
)

// MapKeys returns map keys only
func MapKeys[T cmp.Ordered, V any](data map[T]V) []T {
	keys := make([]T, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	slices.Sort(keys)
	return keys
}

// MergeMapKeys returns map keys only from multiple maps
func MergeMapKeys[V any](m map[string]V, adds ...map[string]V) []string {
	uniq := map[string]bool{}
	for k := range m {
		uniq[k] = true
	}
	for _, add := range adds {
		for k := range add {
			uniq[k] = true
		}
	}

	keys := make([]string, 0, len(uniq))
	for k := range uniq {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}
