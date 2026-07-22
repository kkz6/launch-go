package main

import (
	"github.com/kkz6/launch-go/internal/modules/auth"
	"github.com/kkz6/launch-go/internal/modules/backup"
	"github.com/kkz6/launch-go/internal/modules/billing"
	"github.com/kkz6/launch-go/internal/modules/certificate"
	"github.com/kkz6/launch-go/internal/modules/dashboard"
	databasemodule "github.com/kkz6/launch-go/internal/modules/database"
	"github.com/kkz6/launch-go/internal/modules/dns"
	"github.com/kkz6/launch-go/internal/modules/docker"
	"github.com/kkz6/launch-go/internal/modules/git"
	"github.com/kkz6/launch-go/internal/modules/notification"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/modules/platform"
	"github.com/kkz6/launch-go/internal/modules/script"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/modules/staff"
	wsmodule "github.com/kkz6/launch-go/internal/modules/websocket"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/mail"
)

type moduleSet struct {
	auth         *auth.Module
	server       *server.Module
	database     *databasemodule.Module
	docker       *docker.Module
	site         *site.Module
	dns          *dns.Module
	backup       *backup.Module
	certificate  *certificate.Module
	billing      *billing.Module
	git          *git.Module
	notification *notification.Module
	script       *script.Module
	platform     *platform.Module
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
		auth:         authModule,
		server:       server.NewModule(builder),
		database:     databasemodule.NewModule(builder),
		docker:       docker.NewModule(builder),
		site:         site.NewModule(builder),
		dns:          dns.NewModule(builder),
		backup:       backup.NewModule(builder),
		certificate:  certificate.NewModule(builder),
		billing:      billing.NewModule(builder),
		git:          git.NewModule(builder),
		notification: notificationModule,
		script:       script.NewModule(builder),
		platform:     platform.NewModule(builder),
		dashboard:    dashboard.NewModule(builder),
		staff:        staff.NewModule(builder),
		websocket:    wsmodule.NewModule(builder, a.wsHub),
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
		m.server,
		m.database,
		m.docker,
		m.site,
		m.dns,
		m.backup,
		m.certificate,
		m.billing,
		m.git,
		m.notification,
		m.script,
		m.platform,
		m.dashboard,
		m.staff,
		m.websocket,
	} {
		kernel.Register(module)
	}
}
