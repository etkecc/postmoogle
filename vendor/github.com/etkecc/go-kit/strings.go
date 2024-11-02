package kit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Truncate truncates a string to a specified length, adding "..." if truncated
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

// Unquote is a wrapper around strconv.Unquote, but returns original string if unquoting fails
func Unquote(s string) string {
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return unquoted
}

// Hash returns sha256 hash of a string
func Hash(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// StringToInt converts string to int with optional default value
func StringToInt(value string, optionalDefaultValue ...int) int {
	defaultValue := 0
	if len(optionalDefaultValue) > 0 {
		defaultValue = optionalDefaultValue[0]
	}

	vInt, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return vInt
}

// StringToSlice converts comma-separated string to slice with optional default value
func StringToSlice(value string, optionalDefaultValue ...string) []string {
	var defaultValue string
	if len(optionalDefaultValue) > 0 {
		defaultValue = optionalDefaultValue[0]
	}

	value = strings.TrimSpace(value)
	if idx := strings.Index(value, ","); idx == -1 {
		value = defaultValue
	}

	parts := strings.Split(value, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return parts
}

// SliceToString converts slice of strings into single string (using strings.Join) with optional hook
func SliceToString(slice []string, delimiter string, hook func(string) string) string {
	adjusted := make([]string, 0, len(slice))
	for _, item := range slice {
		if hook != nil {
			item = hook(item)
		}
		adjusted = append(adjusted, item)
	}

	return strings.Join(adjusted, delimiter)
}
