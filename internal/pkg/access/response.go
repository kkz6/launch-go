// Package access provides a generic, domain-agnostic authorization gate.
//
// The gate maps string abilities (e.g. "server.delete") to policy functions
// that decide allow/deny for a given actor and resource. It holds no knowledge
// of HTTP, models, teams, or roles — callers supply those via opaque `any`
// actors and resources, keeping this package a reusable foundation.
package access

// Response is the outcome of an authorization decision: allowed or denied,
// with an optional human-readable reason surfaced to the caller on denial.
type Response struct {
	allowed bool
	message string
}

// Allow returns a permitting response.
func Allow() Response { return Response{allowed: true} }

// Deny returns a refusing response carrying the given reason.
func Deny(message string) Response { return Response{allowed: false, message: message} }

// Allowed reports whether the decision permits the action.
func (r Response) Allowed() bool { return r.allowed }

// Message returns the denial reason (empty when allowed).
func (r Response) Message() string { return r.message }
