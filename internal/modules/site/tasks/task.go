// Package tasks provides site task definitions using factory functions.
//
// Usage:
//
//	task := tasks.Deploy(tasks.DeployConfig{...})
//	// Use with server's TaskRunner for execution
//
// All tasks use the factory function pattern for simplicity and consistency.
package tasks
