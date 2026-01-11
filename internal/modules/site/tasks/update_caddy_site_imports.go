package tasks

import (
	"time"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// SiteInfo contains minimal site information for Caddy imports
type SiteInfo struct {
	ID   string
	Path string
}

// UpdateCaddySiteImports updates the Caddy Sites.caddy file with all site imports
type UpdateCaddySiteImports struct {
	taskrunner.BaseTask
	server *servermodels.Server
	sites  []SiteInfo
}

// NewUpdateCaddySiteImports creates a new UpdateCaddySiteImports task
func NewUpdateCaddySiteImports(server *servermodels.Server, sites []SiteInfo) *UpdateCaddySiteImports {
	return &UpdateCaddySiteImports{
		BaseTask: taskrunner.BaseTask{
			TemplateName: "site/update-caddy-site-imports",
			TaskTimeout:  30 * time.Second,
		},
		server: server,
		sites:  sites,
	}
}

// NewUpdateCaddySiteImportsFromModels creates a task from site models
func NewUpdateCaddySiteImportsFromModels(server *servermodels.Server, sites []models.Site) *UpdateCaddySiteImports {
	siteInfos := make([]SiteInfo, 0, len(sites))

	for _, site := range sites {
		if site.InstalledAt != nil {
			siteInfos = append(siteInfos, SiteInfo{
				ID:   site.ID,
				Path: site.Path,
			})
		}
	}

	return NewUpdateCaddySiteImports(server, siteInfos)
}

// Data returns the template data
func (t *UpdateCaddySiteImports) Data() map[string]interface{} {
	return map[string]interface{}{
		"Sites": t.sites,
	}
}

// Server returns the server for this task
func (t *UpdateCaddySiteImports) Server() *servermodels.Server {
	return t.server
}
