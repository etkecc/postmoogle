// Package envfile loads environment variables from .env files.
package envfile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// DefaultFile is the file Load always reads first.
	DefaultFile = ".env"
	// bom is stripped from the start of a file when present.
	bom = "\ufeff"
)

// Load reads DefaultFile then additionalFiles in order, later files winning.
func Load(additionalFiles ...string) error {
	files := append([]string{DefaultFile}, additionalFiles...)
	errs := make([]error, 0, len(files))

	for _, file := range files {
		if err := loadFile(file); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// loadFile applies every statement it can parse from one file, reporting the rest.
func loadFile(filename string) error {
	fi, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("%s: %w", filename, err)
	}

	if fi.IsDir() {
		return nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("%s: %w", filename, err)
	}

	vars := make(map[string]string)
	parseErr := parseContent(data, vars)
	errs := make([]error, 0, len(vars)+1)
	if parseErr != nil {
		errs = append(errs, parseErr)
	}

	for key, value := range vars {
		if err := os.Setenv(key, value); err != nil {
			errs = append(errs, fmt.Errorf("cannot set %s: %w", key, err))
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s: %w", filename, errors.Join(errs...))
}

// parseContent fills out from src, reporting failed lines without dropping the rest.
func parseContent(src []byte, out map[string]string) error {
	lines := splitLines(bytes.TrimPrefix(src, []byte(bom)))
	errs := make([]error, 0, 1)

	for i, line := range lines {
		if err := parseLine(line, out); err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", i+1, err))
		}
	}

	return errors.Join(errs...)
}

// splitLines breaks src at LF, CRLF and lone CR.
func splitLines(src []byte) [][]byte {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	src = bytes.ReplaceAll(src, []byte("\r"), []byte("\n"))

	return bytes.Split(src, []byte("\n"))
}

// parseLine parses one statement, ignoring blank and comment lines.
func parseLine(line []byte, out map[string]string) error {
	line = bytes.TrimLeftFunc(line, unicode.IsSpace)
	if len(line) == 0 || line[0] == '#' {
		return nil
	}

	key, rest, err := extractKey(line)
	if err != nil {
		return err
	}

	value, err := extractValue(rest, out)
	if err != nil {
		return err
	}

	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("value of %s contains a NUL byte", key)
	}

	out[key] = value

	return nil
}

// extractKey splits a statement into its name and the value that follows.
func extractKey(line []byte) (key string, rest []byte, err error) {
	line = stripExport(bytes.TrimLeftFunc(line, unicode.IsSpace))

	for i := 0; i < len(line); {
		r, size := utf8.DecodeRune(line[i:])
		if r == '=' || r == ':' {
			key = strings.TrimSpace(string(line[:i]))
			if key == "" {
				return "", nil, errors.New("empty variable name")
			}

			return key, bytes.TrimLeftFunc(line[i+size:], unicode.IsSpace), nil
		}

		if !isKeyRune(r) {
			return "", nil, fmt.Errorf("invalid character %q in variable name", r)
		}

		i += size
	}

	return "", nil, errors.New("missing '=' or ':' separator")
}

// stripExport drops a leading "export" keyword.
func stripExport(line []byte) []byte {
	rest, ok := bytes.CutPrefix(line, []byte("export"))
	if !ok || len(rest) == 0 {
		return line
	}

	r, size := utf8.DecodeRune(rest)
	if !unicode.IsSpace(r) {
		return line
	}

	return bytes.TrimLeftFunc(rest[size:], unicode.IsSpace)
}

// isKeyRune reports whether r may appear inside a variable name.
func isKeyRune(r rune) bool {
	return r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r)
}
