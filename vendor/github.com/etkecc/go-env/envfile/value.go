package envfile

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// extractValue reads the value part of a statement, honoring quotes.
func extractValue(src []byte, vars map[string]string) (string, error) {
	if len(src) == 0 {
		return "", nil
	}

	switch src[0] {
	case '\'':
		return extractSingleQuoted(src)
	case '"':
		return extractDoubleQuoted(src, vars)
	default:
		return extractUnquoted(src, vars), nil
	}
}

// extractUnquoted reads a bare value, cutting an inline comment and expanding vars.
func extractUnquoted(src []byte, vars map[string]string) string {
	if idx := commentIndex(src); idx >= 0 {
		src = src[:idx]
	}

	return expand(strings.TrimSpace(string(src)), vars, false)
}

// extractSingleQuoted reads a raw value, which ends at the first quote.
func extractSingleQuoted(src []byte) (string, error) {
	end := bytes.IndexByte(src[1:], '\'')
	if end < 0 {
		return "", errors.New("unterminated single-quoted value")
	}

	if err := checkTail(src[end+2:]); err != nil {
		return "", err
	}

	return string(src[1 : end+1]), nil
}

// extractDoubleQuoted reads an escaped value, expanding vars after decoding escapes.
func extractDoubleQuoted(src []byte, vars map[string]string) (string, error) {
	for i := 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case '"':
			value := expand(string(src[1:i]), vars, true)
			if err := checkTail(src[i+1:]); err != nil {
				return "", err
			}

			return value, nil
		}
	}

	return "", errors.New("unterminated double-quoted value")
}

// checkTail rejects leftover text after a closing quote, allowing a trailing comment.
func checkTail(src []byte) error {
	src = bytes.TrimLeftFunc(src, unicode.IsSpace)
	if len(src) == 0 || src[0] == '#' {
		return nil
	}

	r, _ := utf8.DecodeRune(src)

	return fmt.Errorf("unexpected %q after quoted value", r)
}

// commentIndex returns the offset where an inline comment starts, or -1.
func commentIndex(src []byte) int {
	prevSpace := false

	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if r == '#' && prevSpace {
			return i
		}

		prevSpace = unicode.IsSpace(r)
		i += size
	}

	return -1
}

// expand substitutes $NAME and ${NAME} from vars, keeping unknown references as written.
func expand(v string, vars map[string]string, escapes bool) string {
	var b strings.Builder
	b.Grow(len(v))

	for i := 0; i < len(v); {
		switch {
		case v[i] == '\\' && escapes:
			i += writeEscape(&b, v[i:])
		case v[i] == '\\' && i+1 < len(v) && v[i+1] == '$':
			b.WriteByte('$')
			i += 2
		case v[i] == '$':
			i += writeRef(&b, v[i:], vars)
		default:
			b.WriteByte(v[i])
			i++
		}
	}

	return b.String()
}

// writeEscape decodes one backslash escape into b, returning the bytes consumed.
func writeEscape(b *strings.Builder, v string) int {
	switch v[1] {
	case 'n':
		b.WriteByte('\n')
	case 'r':
		b.WriteByte('\r')
	case 't':
		b.WriteByte('\t')
	case '\\', '"', '$':
		b.WriteByte(v[1])
	default:
		b.WriteString(v[:2])
	}

	return 2
}

// writeRef writes one variable reference into b, returning the bytes consumed.
func writeRef(b *strings.Builder, v string, vars map[string]string) int {
	name, size := refName(v)
	if size == 0 {
		b.WriteByte('$')

		return 1
	}

	if value, ok := vars[name]; ok {
		b.WriteString(value)
	} else {
		b.WriteString(v[:size])
	}

	return size
}

// refName parses $NAME or ${NAME} at the start of v, returning size 0 for neither.
func refName(v string) (name string, size int) {
	if len(v) < 2 {
		return "", 0
	}

	if v[1] != '{' {
		name = v[1 : 1+nameLen(v[1:])]
		if name == "" {
			return "", 0
		}

		return name, 1 + len(name)
	}

	end := strings.IndexByte(v[2:], '}')
	if end < 0 {
		return "", 0
	}

	name = v[2 : end+2]
	if nameLen(name) != len(name) {
		return "", 0
	}

	return name, end + 3
}

// nameLen returns the length of the longest variable-name prefix of s.
func nameLen(s string) int {
	if s == "" || !isNameByte(s[0], 0) {
		return 0
	}

	i := 1
	for i < len(s) && isNameByte(s[i], i) {
		i++
	}

	return i
}

// isNameByte reports whether b may appear in a variable name at offset i, digits after the first.
func isNameByte(b byte, i int) bool {
	switch {
	case b == '_', b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z':
		return true
	case b >= '0' && b <= '9':
		return i > 0
	default:
		return false
	}
}
