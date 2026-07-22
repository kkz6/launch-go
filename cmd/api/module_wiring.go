package main

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/site/adapters"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// wire supplies the small number of dependencies owned by other modules.
func (m moduleSet) wire(a *Application) {
	taskrunner.RegisterHostKeyPersister(func(ctx context.Context, serverID, hostKey string) error {
		return m.server.Repos().Server().UpdateHostKey(ctx, serverID, hostKey)
	})
	m.backup.SetDatabaseRepos(m.database.Repos())
	m.site.SetDomainRepository(m.dns.Repos().Domain())
	m.site.SetProviderFactory(m.git.ProviderFactory())
	m.docker.SetProviderFactory(m.git.ProviderFactory())
	m.site.SetCronCreator(adapters.NewCronCreatorAdapter(m.server.Service()))
	m.site.SetDatabaseManager(adapters.NewDatabaseManagerAdapter(m.database.Service()))
	m.site.SetStoredCertificateRepository(m.certificate.Repos().StoredCertificates)
	m.docker.SetCertificateRepository(m.certificate.Repos())
	m.server.SetSiteReader(m.site.SiteReader())
	m.git.SetSiteChecker(m.site.SiteChecker())
	m.staff.SetServerLogReader(m.server.Repos().Task())
	m.staff.Service().SetInvitationDeps(m.emailSender, a.config.App.Frontend())
	m.auth.Service().Auth.SetPlatformInviteReader(m.staff.Service())
}
