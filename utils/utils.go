package utils

import (
	"strconv"
	"strings"

	"gitlab.com/etke.cc/go/logger"
)

var (
	log     *logger.Logger
	domains []string
)

// SetLogger for utils
func SetLogger(loggerInstance *logger.Logger) {
	log = loggerInstance
}

// SetDomains for later use
func SetDomains(slice []string) {
	domains = slice
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
func EmailsList(mailbox string, domain string) string {
	var msg strings.Builder
	domain = SanitizeDomain(domain)
	msg.WriteString(mailbox)
	msg.WriteString("@")
	msg.WriteString(domain)

	count := len(domains) - 1
	for i, aliasDomain := range domains {
		if i < count {
			msg.WriteString(", ")
		}
		if aliasDomain == domain {
			continue
		}
		msg.WriteString(mailbox)
		msg.WriteString("@")
		msg.WriteString(aliasDomain)
	}

	return msg.String()
}

// Hostname returns hostname part from email address
func Hostname(email string) string {
	return email[strings.LastIndex(email, "@")+1:]
}

// SanitizeDomain checks that input domain is available for use
func SanitizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return domains[0]
	}

	for _, allowed := range domains {
		if domain == allowed {
			return domain
		}
	}

	return domains[0]
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
