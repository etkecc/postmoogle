package utils

import (
	"strconv"
	"strings"

	"gitlab.com/etke.cc/go/logger"
)

var log *logger.Logger

// SetLogger for utils
func SetLogger(loggerInstance *logger.Logger) {
	log = loggerInstance
}

// Mailbox returns mailbox part from email address
func Mailbox(email string) string {
	index := strings.LastIndex(email, "@")
	if index == -1 {
		return email
	}
	return email[:index]
}

// EmailsList returns human-readable list of mailbox's emails for all available domains
func EmailsList(mailbox string, domains []string) string {
	var msg strings.Builder
	count := len(domains) - 1
	for i, domain := range domains {
		msg.WriteString(mailbox)
		msg.WriteString("@")
		msg.WriteString(domain)
		if i < count {
			msg.WriteString(", ")
		}
	}

	return msg.String()
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

// Int converts string to integer
func Int(str string) int {
	if str == "" {
		return 0
	}

	i, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}

	return i
}

// SanitizeBoolString converts string to integer and back to string
func SanitizeIntString(str string) string {
	return strconv.Itoa(Int(str))
}

// StringSlice converts comma-separated string to slice
func StringSlice(str string) []string {
	if str == "" {
		return nil
	}

	str = strings.TrimSpace(str)
	if strings.IndexByte(str, ',') == -1 {
		return []string{str}
	}

	return strings.Split(str, ",")
}

// SanitizeBoolString converts string to slice and back to string
func SanitizeStringSlice(str string) string {
	parts := StringSlice(str)
	if len(parts) == 0 {
		return str
	}

	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return strings.Join(parts, ",")
}
