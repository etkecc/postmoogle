package kit

import "cmp"

// Uniq removes duplicates from slice
func Uniq(slice []string) []string {
	uniq := map[string]struct{}{}
	result := []string{}
	for _, k := range slice {
		if _, ok := uniq[k]; !ok {
			uniq[k] = struct{}{}
			result = append(result, k)
		}
	}

	return result
}

// MergeSlices and remove duplicates
func MergeSlices[K cmp.Ordered](slices ...[]K) []K {
	uniq := make(map[K]struct{}, 0)
	for _, slice := range slices {
		for _, item := range slice {
			uniq[item] = struct{}{}
		}
	}

	return MapKeys(uniq)
}

// RemoveFromSlice removes elements of toRemove from base slice
func RemoveFromSlice[K comparable](base, toRemove []K) []K {
	processed := map[K]struct{}{}
	items := []K{}
	for _, remove := range toRemove {
		processed[remove] = struct{}{}
	}
	for _, item := range base {
		if _, ok := processed[item]; !ok {
			processed[item] = struct{}{}
			items = append(items, item)
		}
	}

	return items
}

// Chunk slices into chunks
func Chunk[T any](items []T, chunkSize int) (chunks [][]T) {
	chunks = make([][]T, 0, (len(items)/chunkSize)+1)
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

// Reverse slice
func Reverse[T any](slice []T) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}
