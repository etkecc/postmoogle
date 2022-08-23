package utils

import (
	"strconv"
	"strings"
)

// Mailbox returns mailbox part from email address
func Mailbox(email string) string {
	index := strings.LastIndex(email, "@")
	if index == -1 {
		return email
	}
	return email[:strings.LastIndex(email, "@")]
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
