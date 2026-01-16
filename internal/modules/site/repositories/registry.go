package repositories

import (
	"gorm.io/gorm"
)

// Registry holds all site module repositories
type Registry struct {
	site        *SiteRepository
	deployment  *DeploymentRepository
	certificate *CertificateRepository
	queue       *QueueRepository
	command     *CommandRepository
	redirect    *RedirectRepository
	release     *ReleaseRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		site:        NewSiteRepository(db),
		deployment:  NewDeploymentRepository(db),
		certificate: NewCertificateRepository(db),
		queue:       NewQueueRepository(db),
		command:     NewCommandRepository(db),
		redirect:    NewRedirectRepository(db),
		release:     NewReleaseRepository(db),
	}
}

// Site returns the site repository
func (r *Registry) Site() *SiteRepository { return r.site }

// Deployment returns the deployment repository
func (r *Registry) Deployment() *DeploymentRepository { return r.deployment }

// Certificate returns the certificate repository
func (r *Registry) Certificate() *CertificateRepository { return r.certificate }

// Queue returns the queue repository
func (r *Registry) Queue() *QueueRepository { return r.queue }

// Command returns the command repository
func (r *Registry) Command() *CommandRepository { return r.command }

// Redirect returns the redirect repository
func (r *Registry) Redirect() *RedirectRepository { return r.redirect }

// Release returns the release repository
func (r *Registry) Release() *ReleaseRepository { return r.release }
