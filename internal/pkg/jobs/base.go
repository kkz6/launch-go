// Package jobs provides unified infrastructure for async job handling.
//
// This package consolidates common patterns used across job implementations,
// reducing boilerplate and ensuring consistency.
//
// # Core Components
//
// Base Context (context.go):
//   - Base: Embeddable struct providing DB, Logger, WS, Queue access
//   - BaseDeps: Dependency container for creating Base contexts
//   - LogInfo, LogError, LogWarn, LogDebug: Convenient logging methods
//   - BroadcastToTeam: WebSocket broadcasting helper
//
// Job Registration (register.go):
//   - RegisterHandler: Generic function to register job handlers with asynq
//   - Handleable: Interface for jobs with Handle() method
//   - Failable: Optional interface for jobs that want failure notification
//
// Payload Helpers (payload.go):
//   - BasePayload: Common fields (UserID, TeamID) for job payloads
//   - ServerPayload: BasePayload + ServerID for server jobs
//   - SitePayload: BasePayload + SiteID for site jobs
//   - PayloadBuilder: Fluent interface for building complex payloads
//
// Task Helpers (helpers.go):
//   - NewTask: Create asynq tasks with type-safe payloads
//   - UnmarshalPayload: Parse asynq task payloads with generics
//   - MustNewTask: Create tasks, panicking on error
//
// Task Builder (builder.go):
//   - TaskBuilder: Fluent interface for building SSH task scripts
//   - QuickTask: Create simple tasks from inline scripts
//   - FileUploadTask, FileDeleteTask, ServiceTask: Common task patterns
//
// # Usage Patterns
//
// Creating a Job Context:
//
//	type JobContext struct {
//	    jobs.Base
//	    Repos *repositories.Registry
//	}
//
//	func NewJobContext(deps jobs.BaseDeps, repos *repositories.Registry) *JobContext {
//	    return &JobContext{
//	        Base:  jobs.NewBase(deps),
//	        Repos: repos,
//	    }
//	}
//
// Defining a Job Payload:
//
//	type InstallCronPayload struct {
//	    jobs.ServerPayload
//	    CronID string `json:"cron_id"`
//	}
//
// Building a Task:
//
//	task := jobs.NewTaskBuilder("Install Cron").
//	    WithTimeoutSeconds(30).
//	    AddHeredoc(cronPath, contents).
//	    AddChmod("644", cronPath).
//	    Build()
package jobs
