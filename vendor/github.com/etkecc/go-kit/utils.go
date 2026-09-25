package kit

import (
	"crypto/subtle"
	"reflect"
)

// Eq compares two strings in constant time. Use it when the strings are secrets.
//
// Plain == bails on the first byte that differs, and that timing difference is a real side
// channel: measure enough comparisons and an attacker walks a token, password, or HMAC out of
// you one byte at a time. Eq checks length and content in constant time and gives that tell
// away. For anything that isn't a secret, == is fine and faster.
func Eq(s1, s2 string) bool {
	b1 := []byte(s1)
	b2 := []byte(s2)
	return subtle.ConstantTimeEq(int32(len(b1)), int32(len(b2))) == 1 && subtle.ConstantTimeCompare(b1, b2) == 1 //nolint:gosec // that's ok
}

// IsNil reports whether v is nil, including the interface that wraps a typed-nil pointer and
// then swears blind it isn't:
//
//	var p *int = nil
//	var i any = p
//	i == nil   // false, the classic Go faceplant
//	IsNil(i)   // true
//
// It covers every nilable kind (pointer, channel, func, map, slice, unsafe pointer) and follows
// pointers down to the bottom. Non-nilable types (int, string, struct) can't be nil, so they
// return false.
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
