package main

import (
	"context"

	"github.com/kkz6/launch-go/internal/middleware"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	backuppolicies "github.com/kkz6/launch-go/internal/modules/backup/policies"
	billingpolicies "github.com/kkz6/launch-go/internal/modules/billing/policies"
	certificatepolicies "github.com/kkz6/launch-go/internal/modules/certificate/policies"
	databasepolicies "github.com/kkz6/launch-go/internal/modules/database/policies"
	dnspolicies "github.com/kkz6/launch-go/internal/modules/dns/policies"
	dockerpolicies "github.com/kkz6/launch-go/internal/modules/docker/policies"
	gitpolicies "github.com/kkz6/launch-go/internal/modules/git/policies"
	notificationpolicies "github.com/kkz6/launch-go/internal/modules/notification/policies"
	platformpolicies "github.com/kkz6/launch-go/internal/modules/platform/policies"
	scriptpolicies "github.com/kkz6/launch-go/internal/modules/script/policies"
	serverpolicies "github.com/kkz6/launch-go/internal/modules/server/policies"
	sitepolicies "github.com/kkz6/launch-go/internal/modules/site/policies"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

func (a *Application) registerRoutes(modules moduleSet) {
	api := a.fiber.Group("")
	api.Get("/health", a.healthCheck)
	authMiddleware := middleware.Auth(a.config.JWT.Secret, a.db)

	middleware.InitTeamMiddleware(a.membershipCache)
	middleware.InitSubscriptionMiddleware(a.db, a.config.Billing.SubscriptionsEnabled)
	middleware.InitServerProvisionedMiddleware(a.db)
	middleware.InitStaffMiddleware(func(userID string) *stafftypes.StaffRole {
		return modules.staff.Service().StaffRoleForUser(context.Background(), userID)
	})
	middleware.InitUserStatus(func(userID string) authtypes.UserStatus {
		return modules.staff.Service().UserStatusForUser(context.Background(), userID)
	})
	middleware.InitSessionValidator(func(sessionID string) bool {
		exists, err := modules.auth.Repos().Session().Exists(context.Background(), sessionID)
		return err == nil && exists
	})

	registerPolicies(modules)
	a.kernel.BootHTTP(app.BootHTTPOptions{Router: api, AuthMiddleware: authMiddleware, TeamContextMiddleware: middleware.TeamScope()})
	modules.server.RegisterRoutes(api, authMiddleware, modules.site.SiteRepository())
	a.kernel.BootWebhooks(a.fiber)
	a.kernel.BootWebSocket(api)
}

func registerPolicies(modules moduleSet) {
	gate := modules.auth.Gate()
	serverpolicies.Register(gate)
	sitepolicies.Register(gate)
	databasepolicies.Register(gate)
	backuppolicies.Register(gate)
	certificatepolicies.Register(gate)
	dnspolicies.Register(gate)
	dockerpolicies.Register(gate)
	gitpolicies.Register(gate)
	scriptpolicies.Register(gate)
	notificationpolicies.Register(gate)
	platformpolicies.Register(gate)
	billingpolicies.Register(gate)
	middleware.InitGate(gate)
}
