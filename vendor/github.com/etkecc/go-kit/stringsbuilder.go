package kit

import "strings"

// StringsBuilder is a wrapper around strings.Builder that provides fluent chaining through shorthand methods.
// The shorthand methods S, B, and R return *StringsBuilder for method chaining, enabling fluent construction
// like sb.S("hello").B(' ').S("world").String(). It also implements io.Writer (via the Write method) and
// io.StringWriter/io.RuneWriter interfaces, making it fully compatible with code expecting those interfaces.
// All methods of strings.Builder are exposed, so StringsBuilder serves as a drop-in replacement.
// The zero value is usable without initialization (strings.Builder's zero value is valid),
// but NewStringsBuilder is provided for clarity.
type StringsBuilder struct {
	b strings.Builder
}

// NewStringsBuilder creates a new StringsBuilder instance. Although the zero value of StringsBuilder
// is usable, this function is provided for code clarity and consistency.
func NewStringsBuilder() StringsBuilder {
	return StringsBuilder{}
}

// S is a shorthand for WriteString that appends the string s to sb's buffer and returns the receiver
// to enable method chaining. It allows fluent construction like sb.S("hello").S(" ").S("world").
func (sb *StringsBuilder) S(s string) *StringsBuilder {
	sb.b.WriteString(s)
	return sb
}

// B is a shorthand for WriteByte that appends the byte b to sb's buffer and returns the receiver
// to enable method chaining. It allows fluent construction like sb.B('x').B('y').
func (sb *StringsBuilder) B(b byte) *StringsBuilder {
	sb.b.WriteByte(b)
	return sb
}

// R is a shorthand for WriteRune that appends the rune r to sb's buffer and returns the receiver
// to enable method chaining. It allows fluent construction like sb.R('α').R('β').
func (sb *StringsBuilder) R(r rune) *StringsBuilder {
	sb.b.WriteRune(r)
	return sb
}

// String returns the accumulated string built by this StringsBuilder.
func (sb *StringsBuilder) String() string {
	return sb.b.String()
}

// Unwrap returns a copy of the underlying strings.Builder (as a value, not a pointer).
// Mutations to the returned builder do not affect the StringsBuilder, as it is a copy.
func (sb *StringsBuilder) Unwrap() strings.Builder {
	return sb.b
}

// WriteString appends the contents of s to sb's buffer and satisfies the io.StringWriter interface.
// It returns the number of bytes written and an error; the error is always nil as strings.Builder never errors.
func (sb *StringsBuilder) WriteString(s string) (int, error) {
	return sb.b.WriteString(s)
}

// Write appends the contents of p to sb's buffer and satisfies the io.Writer interface.
// It returns the number of bytes written and an error; the error is always nil as strings.Builder never errors.
func (sb *StringsBuilder) Write(p []byte) (int, error) {
	return sb.b.Write(p)
}

// WriteRune appends the UTF-8 encoding of the Unicode code point r to sb's buffer and satisfies the io.RuneWriter interface.
// It returns the number of bytes written and an error; the error is always nil as strings.Builder never errors.
func (sb *StringsBuilder) WriteRune(r rune) (int, error) {
	return sb.b.WriteRune(r)
}

// Cap returns the current capacity of the accumulated string buffer.
func (sb *StringsBuilder) Cap() int {
	return sb.b.Cap()
}

// Grow grows sb's capacity, if necessary, to guarantee space for another n bytes and returns the receiver
// to enable method chaining. Unlike strings.Builder.Grow which returns nothing, this method enables
// fluent construction like sb.Grow(10).S("hello").
func (sb *StringsBuilder) Grow(n int) *StringsBuilder {
	sb.b.Grow(n)
	return sb
}

// Len returns the current length of the accumulated string.
func (sb *StringsBuilder) Len() int {
	return sb.b.Len()
}

// Reset resets the StringsBuilder to be empty and returns the receiver to enable method chaining.
// Unlike strings.Builder.Reset which returns nothing, this method enables fluent construction like
// sb.Reset().S("new content").
func (sb *StringsBuilder) Reset() *StringsBuilder {
	sb.b.Reset()
	return sb
}
