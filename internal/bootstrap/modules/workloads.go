// Package modules assembles the feature modules shared by the API and worker.
package modules

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/backup"
	"github.com/kkz6/launch-go/internal/modules/certificate"
	"github.com/kkz6/launch-go/internal/modules/database"
	"github.com/kkz6/launch-go/internal/modules/docker"
	"github.com/kkz6/launch-go/internal/modules/git"
	"github.com/kkz6/launch-go/internal/modules/platform"
	"github.com/kkz6/launch-go/internal/modules/script"
	"github.com/kkz6/launch-go/internal/modules/server"
	"github.com/kkz6/launch-go/internal/modules/site"
	"github.com/kkz6/launch-go/internal/modules/site/adapters"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Workloads contains modules that serve API requests and execute queued work.
type Workloads struct {
	Server      *server.Module
	Database    *database.Module
	Docker      *docker.Module
	Site        *site.Module
	Backup      *backup.Module
	Certificate *certificate.Module
	Git         *git.Module
	Script      *script.Module
	Platform    *platform.Module
}

// NewWorkloads constructs and connects the shared modules before either process
// registers handlers. Process-specific dependencies are supplied by the caller.
func NewWorkloads(builder *app.Builder) Workloads {
	m := Workloads{
		Server:      server.NewModule(builder),
		Database:    database.NewModule(builder),
		Docker:      docker.NewModule(builder),
		Site:        site.NewModule(builder),
		Backup:      backup.NewModule(builder),
		Certificate: certificate.NewModule(builder),
		Git:         git.NewModule(builder),
		Script:      script.NewModule(builder),
		Platform:    platform.NewModule(builder),
	}

	taskrunner.RegisterHostKeyPersister(func(ctx context.Context, serverID, hostKey string) error {
		return m.Server.Repos().Server().UpdateHostKey(ctx, serverID, hostKey)
	})
	m.Backup.SetDatabaseRepos(m.Database.Repos())
	m.Site.SetProviderFactory(m.Git.ProviderFactory())
	m.Docker.SetProviderFactory(m.Git.ProviderFactory())
	m.Site.SetCronCreator(adapters.NewCronCreatorAdapter(m.Server.Service()))
	m.Site.SetDatabaseManager(adapters.NewDatabaseManagerAdapter(m.Database.Service()))
	m.Site.SetStoredCertificateRepository(m.Certificate.Repos().StoredCertificates)
	m.Docker.SetCertificateRepository(m.Certificate.Repos())
	m.Server.SetSiteReader(m.Site.SiteReader())
	m.Git.SetSiteChecker(m.Site.SiteChecker())
	return m
}

// All returns the shared modules in worker registration order.
func (m Workloads) All() []app.Module {
	return []app.Module{
		m.Server,
		m.Database,
		m.Docker,
		m.Backup,
		m.Certificate,
		m.Git,
		m.Site,
		m.Script,
		m.Platform,
	}
}
