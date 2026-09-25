package kit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Truncate cuts s to length runes (runes, not bytes, so it won't slice a multibyte character in
// half) and appends "..." only when it actually cut something. length <= 0 or empty s gives "".
// A string already within length comes back untouched, no ellipsis. Truncate("hello world", 5)
// returns "hello...".
func Truncate(s string, length int) string {
	if length <= 0 || s == "" {
		return ""
	}

	if length >= utf8.RuneCountInString(s) {
		return s
	}

	var buffer bytes.Buffer
	count := 0
	for i := 0; i < len(s); {
		if count >= length {
			break
		}
		r, width := utf8.DecodeRuneInString(s[i:])
		buffer.WriteRune(r)
		i += width
		count++
	}

	truncated := buffer.String()

	if truncated != s {
		return truncated + "..."
	}

	return truncated
}

// Unquote is strconv.Unquote with the error swallowed: it decodes a Go-quoted string, and hands
// the original back unchanged when the input wasn't properly quoted or an escape was bad. Always
// returns a string, never an error.
func Unquote(s string) string {
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return unquoted
}

// Hash returns the SHA-256 of str as lowercase hex, always 64 characters. One-way digest, not
// encryption: there's no getting str back out of it.
func Hash(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// StringToInt parses value as an int, trimming surrounding whitespace first. Empty or unparseable
// falls back to the optional default, or 0 if you didn't pass one. Only the first default counts.
func StringToInt(value string, optionalDefaultValue ...int) int {
	defaultValue := 0
	if len(optionalDefaultValue) > 0 {
		defaultValue = optionalDefaultValue[0]
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}

	vInt, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return vInt
}

// StringToSlice splits a comma-separated value into trimmed parts. Two edge cases worth knowing:
// empty input (after trimming) gives []string{default}, and a non-empty value with no comma gives
// []string{value}, the value itself, not the default. Everything else is split-and-trim.
func StringToSlice(value string, optionalDefaultValue ...string) []string {
	var defaultValue string
	if len(optionalDefaultValue) > 0 {
		defaultValue = optionalDefaultValue[0]
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return []string{defaultValue}
	}
	if !strings.Contains(value, ",") {
		return []string{value}
	}

	parts := strings.Split(value, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return parts
}

// SliceToString joins slice with delimiter, optionally running each element through hook first
// (pass nil to skip it). Empty slice gives "".
func SliceToString(slice []string, delimiter string, hook func(string) string) string {
	if len(slice) == 0 {
		return ""
	}

	adjusted := make([]string, 0, len(slice))
	for _, item := range slice {
		if hook != nil {
			item = hook(item)
		}
		adjusted = append(adjusted, item)
	}

	return strings.Join(adjusted, delimiter)
}
