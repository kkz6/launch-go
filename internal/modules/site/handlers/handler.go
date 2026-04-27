package handlers

import (
	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/services"
)

// Handler aggregates the site-related handlers that still hold bespoke
// routes. The fully-helper-driven sub-resources (commands, redirects,
// queues CRUD, etc.) have no dedicated struct anymore — they are wired
// directly to the framework helpers in routes.go.
type Handler struct {
	Site       *SiteHandler
	Deployment *DeploymentHandler
	SSL        *SSLHandler
	Queue      *QueueHandler
	File       *FileHandler
	Feature    *FeatureHandler
	Webhook    *WebhookHandler
}

// NewHandler creates a new Handler instance with all sub-handlers.
func NewHandler(svc *services.ServiceRegistry) *Handler {
	return &Handler{
		Site:       NewSiteHandler(svc.Site()),
		Deployment: NewDeploymentHandler(svc.Deployment()),
		SSL:        NewSSLHandler(svc.SSL()),
		Queue:      NewQueueHandler(svc.Queue()),
		File:       NewFileHandler(svc.File()),
		Feature:    NewFeatureHandler(svc.Feature()),
		Webhook:    NewWebhookHandler(svc.Deployment()),
	}
}

// SetDomainRepository sets the domain repository for cross-module queries
func (h *Handler) SetDomainRepository(repo dnscontracts.DomainRepository) {
	h.Site.SetDomainRepository(repo)
}
