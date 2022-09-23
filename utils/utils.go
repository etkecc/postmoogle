package utils

import (
	"crypto/subtle"
	"strconv"
	"strings"
)

// Mailbox returns mailbox part from email address
func Mailbox(email string) string {
	index := strings.LastIndex(email, "@")
	if index == -1 {
		return email
	}
	return email[:index]
}

// Hostname returns hostname part from email address
func Hostname(email string) string {
	return email[strings.LastIndex(email, "@")+1:]
}

// Bool converts string to boolean
func Bool(str string) bool {
	str = strings.ToLower(str)
	if str == "" {
		return false
	}

	return (str == "1" || str == "true" || str == "yes")
}

// SanitizeBoolString converts string to boolean and back to string
func SanitizeBoolString(str string) string {
	return strconv.FormatBool(Bool(str))
}

// Compare strings with constant time to prevent timing attacks
func Compare(actual, expected string) bool {
	actualb := []byte(actual)
	expectedb := []byte(expected)

	if expected == "" {
		// Just to keep constant time
		_ = subtle.ConstantTimeCompare(expectedb, expectedb) == 1
		return false
	}

	// actual comparison
	if subtle.ConstantTimeEq(int32(len(actual)), int32(len(expected))) == 1 {
		return subtle.ConstantTimeCompare(actualb, expectedb) == 1
	}

	// Just to keep constant time
	_ = subtle.ConstantTimeCompare(expectedb, expectedb) == 1
	return false
}
