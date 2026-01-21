// Package ptr provides generic pointer utility functions for safe dereferencing
// and pointer creation. These helpers reduce boilerplate when working with
// nullable fields in DTOs, requests, and model mappings.
package util

// Deref safely dereferences a pointer, returning the zero value if nil.
//
// Example:
//
//	name := ptr.Deref(user.Name) // Returns "" if Name is nil
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// DerefOr safely dereferences a pointer, returning the default value if nil.
//
// Example:
//
//	timezone := ptr.DerefOr(server.Timezone, "UTC")
func DerefOr[T any](p *T, defaultVal T) T {
	if p == nil {
		return defaultVal
	}
	return *p
}

// Ptr creates a pointer to the given value.
//
// Example:
//
//	user.Name = ptr.Ptr("John")
func Ptr[T any](v T) *T {
	return &v
}

// NilIfZero returns nil if the value is the zero value, otherwise a pointer.
//
// Example:
//
//	fiberctx.Count = ptr.NilIfZero(count) // nil if count == 0
func NilIfZero[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// NilIfEmpty returns nil if the string is empty, otherwise a pointer.
//
// Example:
//
//	fiberctx.Description = ptr.NilIfEmpty(desc)
func NilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// EmptyIfNil returns empty string if the pointer is nil, otherwise the value.
//
// Example:
//
//	name := ptr.EmptyIfNil(user.Name)
func EmptyIfNil(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Equal compares two pointers for value equality.
// Returns true if both are nil, or if both are non-nil and have equal values.
//
// Example:
//
//	if ptr.Equal(a.Name, b.Name) { ... }
func Equal[T comparable](a, b *T) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// Coalesce returns the first non-nil pointer value, or the zero value if all are nil.
//
// Example:
//
//	name := ptr.Coalesce(user.Nickname, user.Name, ptr.Ptr("Anonymous"))
func Coalesce[T any](ptrs ...*T) T {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	var zero T
	return zero
}

// CoalescePtr returns the first non-nil pointer, or nil if all are nil.
//
// Example:
//
//	namePtr := ptr.CoalescePtr(user.Nickname, user.Name)
func CoalescePtr[T any](ptrs ...*T) *T {
	for _, p := range ptrs {
		if p != nil {
			return p
		}
	}
	return nil
}

// PtrMap applies a function to the dereferenced value if non-nil.
//
// Example:
//
//	upper := util.PtrMap(name, strings.ToUpper) // nil if name is nil
func PtrMap[T, R any](p *T, fn func(T) R) *R {
	if p == nil {
		return nil
	}
	result := fn(*p)
	return &result
}

// PtrMapOr applies a function if non-nil, otherwise returns default.
//
// Example:
//
//	length := util.PtrMapOr(name, len, 0)
func PtrMapOr[T, R any](p *T, fn func(T) R, defaultVal R) R {
	if p == nil {
		return defaultVal
	}
	return fn(*p)
}
