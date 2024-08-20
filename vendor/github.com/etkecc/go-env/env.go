package env

import (
	"os"
	"strconv"
	"strings"

	"github.com/etkecc/go-env/dotenv"
)

var envprefix string

func init() {
	dotenv.Load()
}

// SetPrefix sets prefix for all env vars
func SetPrefix(prefix string) {
	envprefix = prefix
}

// String returns string vars
func String(shortkey string, defaultValue ...string) string {
	var dv string
	if len(defaultValue) > 0 {
		dv = defaultValue[0]
	}

	key := strings.ToUpper(envprefix + "_" + strings.ReplaceAll(shortkey, ".", "_"))
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return dv
	}

	return value
}

// Int returns int vars
func Int(shortkey string, defaultValue ...int) int {
	var dv int
	if len(defaultValue) > 0 {
		dv = defaultValue[0]
	}

	str := String(shortkey)
	if str == "" {
		return dv
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		return dv
	}

	return val
}

// Bool returns boolean vars (1, true, yes)
func Bool(shortkey string) bool {
	str := strings.ToLower(String(shortkey))
	if str == "" {
		return false
	}
	return (str == "1" || str == "true" || str == "yes")
}

// Slice returns slice from space-separated strings, eg: export VAR="one two three" => []string{"one", "two", "three"}
func Slice(shortkey string) []string {
	str := String(shortkey)
	if str == "" {
		return nil
	}

	return strings.Split(str, " ")
}
