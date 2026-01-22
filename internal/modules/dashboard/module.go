package dashboard

import (
	authrepos "github.com/kkz6/launch-go/internal/modules/auth/repositories"
	backuprepos "github.com/kkz6/launch-go/internal/modules/backup/repositories"
	"github.com/kkz6/launch-go/internal/modules/dashboard/services"
	dnsrepos "github.com/kkz6/launch-go/internal/modules/dns/repositories"
	gitrepos "github.com/kkz6/launch-go/internal/modules/git/repositories"
	notificationrepos "github.com/kkz6/launch-go/internal/modules/notification/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

const ModuleName = "dashboard"

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
)

// Module represents the dashboard module
type Module struct {
	app.Base
	service *services.DashboardService
}

// NewModule creates a new dashboard module
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()

	repos := &services.Repositories{
		User:                authrepos.NewUserRepository(deps.DB),
		ServerProvider:      serverrepos.NewServerProviderRepository(deps.DB),
		SourceControl:       gitrepos.NewSourceControlRepository(deps.DB),
		DomainProvider:      dnsrepos.NewDomainProviderRepository(deps.DB),
		StorageProvider:     backuprepos.NewStorageProviderRepository(deps.DB),
		NotificationChannel: notificationrepos.NewNotificationChannelRepository(deps.DB),
	}

	service := services.NewDashboardService(deps.DB, deps.Logger, repos)

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		service: service,
	}
}
