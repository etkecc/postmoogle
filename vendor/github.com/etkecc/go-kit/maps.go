package kit

import (
	"cmp"
	"slices"
	"sort"
)

// MapFromSlice creates a map from slice elements as keys
// The map values are set to true, indicating the presence of the key.
// This is useful for quickly checking if a key exists in the map.
func MapFromSlice[T cmp.Ordered](slice []T) map[T]bool {
	data := make(map[T]bool, len(slice))
	for _, k := range slice {
		data[k] = true
	}
	return data
}

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
