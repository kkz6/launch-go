// Package jobs provides base infrastructure for async job handling
package jobs

// This package provides:
// - RegisterHandler: Generic function to register job handlers (register.go)
// - Handleable: Interface for jobs with Handle() method (register.go)
// - Failable: Optional interface for jobs that want failure notification (register.go)
// - NewTask/UnmarshalPayload: Task creation and payload parsing helpers (helpers.go)
// - Broadcaster: Interface for websocket broadcasting (context.go)
// - Various tracking utilities (tracking.go)
// - Common interfaces for type safety (interfaces.go)
