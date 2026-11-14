package main

import (
	"github.com/kkz6/launch-go/internal/bootstrap/modules"
	"github.com/kkz6/launch-go/internal/modules/auth"
	"github.com/kkz6/launch-go/internal/modules/billing"
	"github.com/kkz6/launch-go/internal/modules/dashboard"
	"github.com/kkz6/launch-go/internal/modules/dns"
	"github.com/kkz6/launch-go/internal/modules/notification"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/staff"
	wsmodule "github.com/kkz6/launch-go/internal/modules/websocket"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/mail"
)

type moduleSet struct {
	modules.Workloads
	auth         *auth.Module
	dns          *dns.Module
	billing      *billing.Module
	notification *notification.Module
	dashboard    *dashboard.Module
	staff        *staff.Module
	websocket    *wsmodule.Module
	emailSender  channels.EmailSender
}

func (a *Application) newModules() moduleSet {
	emailSender := mail.NewEmailSender(a.config.Mail)
	if emailSender == nil {
		a.logger.Warn().Str("driver", a.config.Mail.Driver).Msg("Email sender not configured — team invitations, password resets, and notifications will not send emails. Set RESEND_API_KEY or SMTP_HOST to enable.")
	}

	builder := a.newModuleBuilder()
	notificationModule := notification.NewModule(builder, emailSender)
	builder = builder.WithNotifier(notificationModule.Notifier())
	authModule, err := auth.NewModule(builder, emailSender, a.redisCache)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("Failed to create auth module")
	}

	return moduleSet{
		Workloads:    modules.NewWorkloads(builder),
		auth:         authModule,
		dns:          dns.NewModule(builder),
		billing:      billing.NewModule(builder),
		notification: notificationModule,
		dashboard:    dashboard.NewModule(builder),
		staff:        staff.NewModule(builder),
		websocket:    wsmodule.NewModule(builder, a.wsHub, authModule.Gate()),
		emailSender:  emailSender,
	}
}

func (a *Application) newModuleBuilder() *app.Builder {
	return app.NewBuilder(app.Deps{
		Config:          a.config,
		DB:              a.db,
		Logger:          a.logger,
		Queue:           a.queueClient,
		WebSocket:       a.broadcaster,
		Dispatcher:      a.dispatcher,
		MembershipCache: a.membershipCache,
	})
}

func (m moduleSet) register(kernel *app.Kernel) {
	for _, module := range []app.Module{
		m.auth,
		m.Server,
		m.Database,
		m.Docker,
		m.Site,
		m.dns,
		m.Backup,
		m.Certificate,
		m.billing,
		m.Git,
		m.notification,
		m.Script,
		m.Platform,
		m.dashboard,
		m.staff,
		m.websocket,
	} {
		kernel.Register(module)
	}
}
