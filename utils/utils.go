package utils

import (
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

var (
	log     *zerolog.Logger
	domains []string
)

// SetLogger for utils
func SetLogger(loggerInstance *zerolog.Logger) {
	log = loggerInstance
}

// SetDomains for later use
func SetDomains(slice []string) {
	domains = slice
}

// AddrIP returns IP from a network address
func AddrIP(addr net.Addr) string {
	key := addr.String()
	host, _, _ := net.SplitHostPort(key) //nolint:errcheck // either way it's ok
	if host != "" {
		key = host
	}
	return key
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

// Int64 converts string into int64
func Int64(str string) int64 {
	if str == "" {
		return 0
	}

	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// SanitizeBoolString converts string to integer and back to string
func SanitizeIntString(str string) string {
	return strconv.Itoa(Int(str))
}

// SliceString converts slice into comma-separated string
func SliceString(strs []string) string {
	res := []string{}
	for _, str := range strs {
		str = strings.TrimSpace(str)
		if str == "" {
			continue
		}
		res = append(res, str)
	}
	sort.Strings(res)
	return strings.Join(res, ",")
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

	parts := strings.Split(str, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return parts
}

// SanitizeBoolString converts string to slice and back to string
func SanitizeStringSlice(str string) string {
	return SliceString(StringSlice(str))
}
