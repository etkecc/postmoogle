package env

import (
	"os"
	"strconv"
	"strings"
)

var envprefix string

// SetPrefix sets prefix for all env vars
func SetPrefix(prefix string) {
	envprefix = prefix
}

// String returns string vars
func String(shortkey string, defaultValue string) string {
	key := strings.ToUpper(envprefix + "_" + strings.ReplaceAll(shortkey, ".", "_"))
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	return value
}

// Int returns int vars
func Int(shortkey string, defaultValue int) int {
	str := String(shortkey, "")
	if str == "" {
		return defaultValue
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}

	return val
}

// Bool returns boolean vars (1, true, yes)
func Bool(shortkey string) bool {
	str := strings.ToLower(String(shortkey, ""))
	if str == "" {
		return false
	}
	return (str == "1" || str == "true" || str == "yes")
}

// Slice returns slice from space-separated strings, eg: export VAR="one two three" => []string{"one", "two", "three"}
func Slice(shortkey string) []string {
	str := String(shortkey, "")
	if str == "" {
		return nil
	}

	return strings.Split(str, " ")
}
