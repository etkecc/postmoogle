// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package exstrings

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"
	"unsafe"
)

// UnsafeBytes returns a byte slice that points to the same memory as the input string.
//
// The returned byte slice must not be modified.
func UnsafeBytes(str string) []byte {
	return unsafe.Slice(unsafe.StringData(str), len(str))
}

// SHA256 returns the SHA-256 hash of the input string without copying the string.
func SHA256(str string) [32]byte {
	return sha256.Sum256(UnsafeBytes(str))
}

// ConstantTimeEqual compares two strings using [subtle.ConstantTimeCompare] without copying the strings.
//
// Note that ConstantTimeCompare is not constant time if the strings are of different length.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare(UnsafeBytes(a), UnsafeBytes(b)) == 1
}

// LongestSequenceOf returns the length of the longest contiguous sequence of a single rune in a string.
func LongestSequenceOf(a string, b rune) int {
	// IndexRune has some optimizations, so use it to find the starting point
	firstIndex := strings.IndexRune(a, b)
	if firstIndex == -1 {
		return 0
	}
	count := 0
	maxCount := 0
	for _, r := range a[firstIndex:] {
		if r == b {
			count++
			if count > maxCount {
				maxCount = count
			}
		} else {
			count = 0
		}
	}
	return maxCount
}

// LongestSequenceOfFunc returns the length of the longest contiguous sequence of runes in a string.
//
// If the provided function returns zero or higher, the return value is added to the current count.
// If the return value is negative, the count is reset to zero.
func LongestSequenceOfFunc(a string, fn func(b rune) int) int {
	count := 0
	maxCount := 0
	for _, r := range a {
		val := fn(r)
		if val < 0 {
			count = 0
		} else {
			count += val
			if count > maxCount {
				maxCount = count
			}
		}
	}
	return maxCount
}
