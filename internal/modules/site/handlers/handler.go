package handlers

import (
	dnscontracts "github.com/kkz6/launch-go/internal/modules/dns/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/services"
)

// Handler aggregates all site-related handlers
type Handler struct {
	Site       *SiteHandler
	Deployment *DeploymentHandler
	SSL        *SSLHandler
	Queue      *QueueHandler
	Command    *CommandHandler
	Redirect   *RedirectHandler
	File       *FileHandler
	Webhook    *WebhookHandler
}

// NewHandler creates a new Handler instance with all sub-handlers
func NewHandler(svc *services.ServiceRegistry) *Handler {
	return &Handler{
		Site:       NewSiteHandler(svc.Site()),
		Deployment: NewDeploymentHandler(svc.Deployment()),
		SSL:        NewSSLHandler(svc.SSL()),
		Queue:      NewQueueHandler(svc.Queue()),
		Command:    NewCommandHandler(svc.Command()),
		Redirect:   NewRedirectHandler(svc.Redirect()),
		File:       NewFileHandler(svc.File()),
		Webhook:    NewWebhookHandler(svc.Deployment()),
	}
}

// SetDomainRepository sets the domain repository for cross-module queries
func (h *Handler) SetDomainRepository(repo dnscontracts.DomainRepository) {
	h.Site.SetDomainRepository(repo)
}
