package kit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Truncate truncates a string to a specified length measured in Unicode code points (runes),
// appending "..." if truncation occurs. Length is measured in runes, not bytes.
// Returns empty string if length <= 0 or s is empty. The ellipsis is appended only when
// actual truncation occurs; if the string is already shorter than length, it is returned unchanged.
// Example: Truncate("hello world", 5) returns "hello...".
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

// Unquote handles Go-style quoted strings by wrapping strconv.Unquote.
// It decodes escape sequences in quoted strings (e.g., "hello\nworld" becomes hello with a newline).
// Returns the original string unchanged if the input is not properly quoted or if any escape
// sequence is invalid. Never returns an error; always returns a string.
func Unquote(s string) string {
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return unquoted
}

// Hash computes the SHA-256 hash of a string and returns it as a lowercase hex-encoded string.
// The result is always 64 characters long. This is a one-way cryptographic hash, not encryption.
func Hash(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// StringToInt converts a string to an integer. Leading and trailing whitespace is trimmed
// before parsing. Returns the provided default value (or 0 if not provided) on empty string
// or parse failure. Accepts an optional variadic parameter for the default value;
// if multiple defaults are provided, only the first is used.
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

// StringToSlice converts a comma-separated string into a slice of strings.
// Each element is whitespace-trimmed. Accepts an optional variadic parameter for a default value.
// An empty input (after trimming) returns []string{defaultValue}.
// A non-empty input with no comma returns []string{value} — the value itself, not the default.
// A comma-separated input is split and each element is trimmed individually.
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

// SliceToString joins a slice of strings into a single string using the provided delimiter.
// An optional hook function can be applied to transform each element before joining.
// Pass nil for the hook parameter to skip transformation. Returns an empty string if
// the slice is empty.
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
