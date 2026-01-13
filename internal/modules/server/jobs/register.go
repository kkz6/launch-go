package jobs

import (
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

// jobContext is the shared job context for all server jobs
var jobContext *JobContext

// SetJobContext sets the shared job context for all server jobs
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

// Register registers all server jobs with the job registry.
// This function is called by the kernel to register module jobs.
//
// Usage in kernel:
//
//	serverjobs.SetJobContext(ctx)
//	kernel.RegisterModule(serverjobs.Register)
func Register(r *jobs.Registry) {
	r.Register(TypeProvisionServer, newProvisionServerJob)
	r.Register(TypeInstallCron, newInstallCronJob)
	r.Register(TypeUninstallCron, newUninstallCronJob)
	r.Register(TypeInstallDaemon, newInstallDaemonJob)
	r.Register(TypeUninstallDaemon, newUninstallDaemonJob)
	r.Register(TypeRestartDaemon, newRestartDaemonJob)
	r.Register(TypeInstallFirewallRule, newInstallFirewallRuleJob)
	r.Register(TypeUninstallFirewall, newUninstallFirewallRuleJob)
	r.Register(TypeAddSshKey, newAddSshKeyJob)
	r.Register(TypeRemoveSshKey, newRemoveSshKeyJob)
}

// Factory functions for each job type

func newProvisionServerJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[ProvisionServerPayload](t)
	if err != nil {
		return nil, err
	}
	job := &ProvisionServerJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newInstallCronJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[InstallCronPayload](t)
	if err != nil {
		return nil, err
	}
	job := &InstallCronJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newUninstallCronJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[UninstallCronPayload](t)
	if err != nil {
		return nil, err
	}
	job := &UninstallCronJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newInstallDaemonJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[InstallDaemonPayload](t)
	if err != nil {
		return nil, err
	}
	job := &InstallDaemonJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newUninstallDaemonJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[UninstallDaemonPayload](t)
	if err != nil {
		return nil, err
	}
	job := &UninstallDaemonJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newRestartDaemonJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[RestartDaemonPayload](t)
	if err != nil {
		return nil, err
	}
	job := &RestartDaemonJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newInstallFirewallRuleJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[InstallFirewallRulePayload](t)
	if err != nil {
		return nil, err
	}
	job := &InstallFirewallRuleJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newUninstallFirewallRuleJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[UninstallFirewallRulePayload](t)
	if err != nil {
		return nil, err
	}
	job := &UninstallFirewallRuleJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newAddSshKeyJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[AddSshKeyPayload](t)
	if err != nil {
		return nil, err
	}
	job := &AddSshKeyJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}

func newRemoveSshKeyJob(t *asynq.Task) (jobs.Handler, error) {
	payload, err := jobs.ParsePayload[RemoveSshKeyPayload](t)
	if err != nil {
		return nil, err
	}
	job := &RemoveSshKeyJob{Payload: payload}
	job.SetContext(jobContext)
	return job, nil
}
