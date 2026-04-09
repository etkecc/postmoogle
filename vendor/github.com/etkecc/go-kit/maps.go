package kit

import (
	"cmp"
	"slices"
	"sort"
)

// MapFromSlice creates a map from slice elements as keys,
// enabling O(1) membership testing. The map values are set to true,
// indicating the presence of the key. This is useful for converting
// a slice to a set for quick lookups.
//
// Duplicate elements in the input are silently collapsed, with each
// unique element appearing once in the resulting map. An empty or nil
// slice returns an empty map (not nil).
func MapFromSlice[T cmp.Ordered](slice []T) map[T]bool {
	data := make(map[T]bool, len(slice))
	for _, k := range slice {
		data[k] = true
	}
	return data
}

// MapKeys returns the keys of the provided map as a sorted (ascending)
// slice. An empty or nil map returns an empty slice (not nil).
// This function is used internally by List.Slice().
func MapKeys[T cmp.Ordered, V any](data map[T]V) []T {
	keys := make([]T, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	slices.Sort(keys)
	return keys
}

// MergeMapKeys deduplicates keys across all provided maps and returns
// them as a sorted (ascending) slice. The first argument m is required;
// adds is variadic and may be empty. The return is always a non-nil slice,
// which will be empty if all provided maps are empty.
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
