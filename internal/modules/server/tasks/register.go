package tasks

// RegisterTaskCallbacks registers all server task callback handlers.
// This should be called during application startup.
//
// Currently, server tasks don't have callbacks, but this placeholder
// is here for consistency with site/tasks/register.go
func RegisterTaskCallbacks() {
	// Server task callbacks will be registered here when needed
	// Example:
	// taskrunner.RegisterCallback(ProvisionServerTaskType, NewProvisionServerCallback)
}
