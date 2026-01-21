// Package crypto provides cryptographic utilities for secure operations.
package crypto

import (
	"crypto/subtle"
)

// SecureCompare performs a constant-time comparison of two strings.
// This prevents timing attacks where an attacker could determine
// how many characters match by measuring response time.
//
// Returns true if the strings are equal, false otherwise.
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SecureCompareBytes performs a constant-time comparison of two byte slices.
// This prevents timing attacks where an attacker could determine
// how many bytes match by measuring response time.
//
// Returns true if the byte slices are equal, false otherwise.
func SecureCompareBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
