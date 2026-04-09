package kit

import (
	"crypto/subtle"
	"reflect"
)

// Eq checks if 2 strings are equal in constant time.
//
// This function uses constant-time comparison to prevent timing attacks.
// Unlike the == operator, which short-circuits and leaks timing information
// character by character, Eq compares both the length and content in constant
// time. This prevents attackers from inferring secret values (such as tokens,
// passwords, or HMACs) by measuring how long the comparison takes.
//
// When comparing secrets, always use Eq instead of s1 == s2. For non-secret
// string comparisons where timing doesn't matter, == is fine.
func Eq(s1, s2 string) bool {
	b1 := []byte(s1)
	b2 := []byte(s2)
	return subtle.ConstantTimeEq(int32(len(b1)), int32(len(b2))) == 1 && subtle.ConstantTimeCompare(b1, b2) == 1 //nolint:gosec // that's ok
}

// IsNil checks if the given value is nil, including interfaces holding nil pointers.
//
// Go has a subtle pitfall with interfaces: a typed nil pointer wrapped in an
// interface{} is NOT equal to nil via a plain == check. For example:
//
//	var p *int = nil
//	var i any = p
//	i == nil          // false (interface{} holds a non-nil interface value)
//	IsNil(i)          // true (correctly identifies the nil pointer)
//
// IsNil handles all nilable kinds: pointers, channels, functions, maps, slices,
// and unsafe pointers. It also traverses through pointer layers to detect nil at
// any depth. For non-nilable types (int, string, struct, etc.), it returns false.
//
// Use this function when you need to check for nil in a way that accounts for
// interface{} wrapping and polymorphic types.
func IsNil(i any) bool {
	// standard nil check
	if i == nil {
		return true
	}

	// special case: interfaces holding nil pointers (e.g. var x *int = nil; var y any = x)
	rv := reflect.ValueOf(i)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return true
		}
		rv = rv.Elem()
	}

	// check for other kinds that can be nil
	switch rv.Kind() {
	case reflect.Chan,
		reflect.Func,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice,
		reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}
