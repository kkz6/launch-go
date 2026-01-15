package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// Task creation functions - these create asynq tasks that can be enqueued

// NewRebootServerTask creates an asynq task for rebooting a server
func NewRebootServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRebootServer, RebootServerPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}

// NewAddServiceTask creates an asynq task for adding a service
func NewAddServiceTask(serverID, serviceID string, software enums.Software) (*asynq.Task, error) {
	return jobs.NewTask(TypeAddService, AddServicePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Software:  string(software),
	})
}

// NewRemoveServiceTask creates an asynq task for removing a service
func NewRemoveServiceTask(serverID, serviceID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeRemoveService, RemoveServicePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		UserID:    userID,
	})
}

// NewServiceOperationTask creates an asynq task for service operations
func NewServiceOperationTask(serverID, serviceID, operation string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeServiceOperation, ServiceOperationPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Operation: operation,
		UserID:    userID,
	})
}

// NewUpdateConnectivityTask creates an asynq task for updating server connectivity
func NewUpdateConnectivityTask(serverID string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUpdateConnectivity, UpdateConnectivityPayload{
		ServerID: serverID,
	})
}

// NewArchiveServerTask creates an asynq task for archiving a server
func NewArchiveServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeArchiveServer, ArchiveServerPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}

// NewUnarchiveServerTask creates an asynq task for unarchiving a server
func NewUnarchiveServerTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeUnarchiveServer, UnarchiveServerPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}

// NewConfigureOpcacheTask creates an asynq task for configuring OPcache
func NewConfigureOpcacheTask(serverID, serviceID string, settings map[string]string) (*asynq.Task, error) {
	return jobs.NewTask(TypeConfigureOpcache, ConfigureOpcachePayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		Settings:  settings,
	})
}

// NewVulnerabilityAuditTask creates an asynq task for running a vulnerability audit
func NewVulnerabilityAuditTask(serverID, teamID string, userID, emailRecipient *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeVulnerabilityAudit, VulnerabilityAuditPayload{
		ServerID:       serverID,
		TeamID:         teamID,
		UserID:         userID,
		EmailRecipient: emailRecipient,
	})
}

// NewCheckServiceStatusTask creates an asynq task for checking a service status
func NewCheckServiceStatusTask(serverID, serviceID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeCheckServiceStatus, CheckServiceStatusPayload{
		ServerID:  serverID,
		ServiceID: serviceID,
		UserID:    userID,
	})
}

// NewSyncDaemonsTask creates an asynq task for syncing daemon status
func NewSyncDaemonsTask(serverID string, userID *string) (*asynq.Task, error) {
	return jobs.NewTask(TypeSyncDaemons, SyncDaemonsPayload{
		ServerID: serverID,
		UserID:   userID,
	})
}
