// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package exslices

// DeduplicateUnsorted removes duplicates from the given slice without requiring that the input slice is sorted.
// The order of the output will be the same as the input.
//
// If you don't care about the order of the output, it's recommended to sort the list and then use [slices.Compact].
func DeduplicateUnsorted[T comparable](s []T) []T {
	seen := make(map[T]struct{}, len(s))
	result := make([]T, 0, len(s))
	for _, item := range s {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
