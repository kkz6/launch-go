package jobs

import (
	"github.com/kkz6/launch-go/internal/pkg/jobs"
)

var jobContext *JobContext

// SetJobContext sets the global job context
func SetJobContext(ctx *JobContext) {
	jobContext = ctx
}

func getContext() any {
	return jobContext
}

// Register registers all site jobs with the registry
func Register(r *jobs.Registry) {
	// Command jobs
	r.Register(TypeRunCommand, jobs.MakeFactory(
		func(p RunCommandPayload) *RunCommandJob { return &RunCommandJob{Payload: p} },
		getContext,
	))

	// Deployment jobs
	r.Register(TypeDeploy, jobs.MakeFactory(
		func(p DeployPayload) *DeployJob { return &DeployJob{Payload: p} },
		getContext,
	))

	r.Register(TypeDeployZeroDowntime, jobs.MakeFactory(
		func(p DeployPayload) *DeployZeroDowntimeJob { return &DeployZeroDowntimeJob{Payload: p} },
		getContext,
	))

	r.Register(TypeRollback, jobs.MakeFactory(
		func(p RollbackPayload) *RollbackJob { return &RollbackJob{Payload: p} },
		getContext,
	))

	// SSL jobs (TODO: implement handler)
	r.Register(TypeInstallSSL, jobs.MakeFactory(
		func(p InstallSSLPayload) *InstallSSLJob { return &InstallSSLJob{Payload: p} },
		getContext,
	))

	// Caddyfile jobs
	r.Register(TypeInstallCaddyfile, jobs.MakeFactory(
		func(p CaddyfilePayload) *InstallCaddyfileJob { return &InstallCaddyfileJob{Payload: p} },
		getContext,
	))

	r.Register(TypeUpdateCaddyfile, jobs.MakeFactory(
		func(p CaddyfilePayload) *UpdateCaddyfileJob { return &UpdateCaddyfileJob{Payload: p} },
		getContext,
	))

	r.Register(TypeUninstallCaddyfile, jobs.MakeFactory(
		func(p CaddyfilePayload) *UninstallCaddyfileJob { return &UninstallCaddyfileJob{Payload: p} },
		getContext,
	))
}
