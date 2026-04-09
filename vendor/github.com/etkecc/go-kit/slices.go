package kit

import "cmp"

// Uniq removes duplicates from the input slice while preserving insertion order.
// The first occurrence of each element is kept, and subsequent duplicates are discarded.
// Unlike MergeSlices, the result maintains the order of first appearance rather than being sorted.
// A nil or empty input slice returns an empty non-nil slice.
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

// MergeSlices concatenates any number of input slices, deduplicates the result,
// and returns the elements in sorted order. The order of input slices does not affect
// the output order, as the result is always sorted via MapKeys. Empty or nil input
// slices are silently ignored. The generic type K must be ordered (implements cmp.Ordered).
func MergeSlices[K cmp.Ordered](slices ...[]K) []K {
	uniq := make(map[K]struct{}, 0)
	for _, slice := range slices {
		for _, item := range slice {
			uniq[item] = struct{}{}
		}
	}

	return MapKeys(uniq)
}

// RemoveFromSlice returns a new slice containing elements from base that do not
// appear in the toRemove slice, preserving the order of elements from base.
// The result is also deduplicated: if an element appears multiple times in base,
// it appears only once in the output. A nil base slice returns an empty slice.
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

// Chunk divides items into sub-slices of at most chunkSize elements each.
// The last chunk may be smaller than chunkSize if the total number of items
// is not evenly divisible. Note: chunkSize must be greater than 0; values <= 0
// will cause an infinite loop. A nil or empty input slice returns a single empty chunk.
func Chunk[T any](items []T, chunkSize int) (chunks [][]T) {
	chunks = make([][]T, 0, (len(items)/chunkSize)+1)
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

// Reverse reverses the elements of slice in place without allocating new memory.
// For a nil or empty slice, this is a no-op.
func Reverse[T any](slice []T) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}
