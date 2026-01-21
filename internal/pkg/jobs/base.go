// Package jobs provides base infrastructure for async job handling
package jobs

// This package provides:
// - Base: Embeddable base struct for job contexts with logging/broadcasting (context.go)
// - RegisterHandler: Generic function to register job handlers (register.go)
// - Handleable: Interface for jobs with Handle() method (register.go)
// - Failable: Optional interface for jobs that want failure notification (register.go)
// - NewTask/UnmarshalPayload: Task creation and payload parsing helpers (helpers.go)
// - Common interfaces for type safety (interfaces.go)
