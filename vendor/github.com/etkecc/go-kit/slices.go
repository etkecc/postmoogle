package kit

import "cmp"

// Uniq drops duplicates while keeping first-seen order: the first copy of each element stays,
// later ones vanish. (MergeSlices sorts instead; this one preserves the order you handed it.)
// Nil or empty in, empty non-nil slice out.
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

// MergeSlices concatenates every input slice, dedupes, and returns the result sorted ascending,
// so input order doesn't survive. Empty and nil inputs are skipped. K has to be cmp.Ordered,
// since the output comes back sorted.
func MergeSlices[K cmp.Ordered](slices ...[]K) []K {
	uniq := make(map[K]struct{}, 0)
	for _, slice := range slices {
		for _, item := range slice {
			uniq[item] = struct{}{}
		}
	}

	return MapKeys(uniq)
}

// RemoveFromSlice returns the elements of base that aren't in toRemove, in base's order,
// deduplicated so a repeat in base still shows up only once. Nil base gives an empty slice.
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

// Chunk splits items into sub-slices of at most chunkSize each; the last one holds whatever's
// left over. A non-positive chunkSize means "don't chunk": the whole slice comes back as one
// chunk, no panic, no divide-by-zero, no drama. Nil or empty input gives a single empty chunk.
func Chunk[T any](items []T, chunkSize int) (chunks [][]T) {
	if chunkSize <= 0 {
		return [][]T{items}
	}
	chunks = make([][]T, 0, (len(items)/chunkSize)+1)
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

// Reverse flips slice end to end in place, no allocation. No-op on nil or empty.
func Reverse[T any](slice []T) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}
