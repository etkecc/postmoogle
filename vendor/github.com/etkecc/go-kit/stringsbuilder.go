package kit

import "strings"

// StringsBuilder is a wrapper around strings.Builder with short syntax,
// it also exposes all methods of strings.Builder, so you can use it as a drop-in replacement
type StringsBuilder struct {
	b strings.Builder
}

// NewStringsBuilder creates a new StringsBuilder instance
func NewStringsBuilder() StringsBuilder {
	return StringsBuilder{}
}

// S is shortcut for WriteString
func (sb *StringsBuilder) S(s string) *StringsBuilder {
	sb.b.WriteString(s)
	return sb
}

// B is shortcut for WriteByte
func (sb *StringsBuilder) B(b byte) *StringsBuilder {
	sb.b.WriteByte(b)
	return sb
}

// R is shortcut for WriteRune
func (sb *StringsBuilder) R(r rune) *StringsBuilder {
	sb.b.WriteRune(r)
	return sb
}

// String returns the accumulated string
func (sb *StringsBuilder) String() string {
	return sb.b.String()
}

// Unwrap returns the underlying strings.Builder
func (sb *StringsBuilder) Unwrap() strings.Builder {
	return sb.b
}

// WriteString appends the contents of s to sb's buffer
func (sb *StringsBuilder) WriteString(s string) (int, error) {
	return sb.b.WriteString(s)
}

// Write appends the contents of p to sb's buffer
func (sb *StringsBuilder) Write(p []byte) (int, error) {
	return sb.b.Write(p)
}

// WriteRune appends the UTF-8 encoding of Unicode code point r to sb's buffer
func (sb *StringsBuilder) WriteRune(r rune) (int, error) {
	return sb.b.WriteRune(r)
}

// Cap returns the current capacity of the accumulated string
func (sb *StringsBuilder) Cap() int {
	return sb.b.Cap()
}

// Grow grows sb's capacity, if necessary, to guarantee space for another n bytes
func (sb *StringsBuilder) Grow(n int) *StringsBuilder {
	sb.b.Grow(n)
	return sb
}

// Len returns the current length of the accumulated string
func (sb *StringsBuilder) Len() int {
	return sb.b.Len()
}

// Reset resets the StringsBuilder to be empty and returns itself
func (sb *StringsBuilder) Reset() *StringsBuilder {
	sb.b.Reset()
	return sb
}
