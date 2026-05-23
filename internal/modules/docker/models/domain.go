package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationDomain attaches a public hostname to a docker application.
// Traefik routes requests for this host to the application's container.
// Each domain is independent — adding/removing one rewrites the Traefik
// dynamic-config file for the app, no redeploy required.
//
// Field set mirrors dokploy's Domain model so the UI can reuse the
// same dialog shape:
//
//   - Host              — public hostname (api.example.com)
//   - Path              — external URL path Traefik routes from
//                          ("/api"). Empty = match all paths.
//   - InternalPath      — path the application expects internally
//                          ("/"). When ExternalPath != InternalPath,
//                          Traefik rewrites between them.
//   - StripPath         — when true, strip Path before forwarding to
//                          the container. Useful for apps mounted at
//                          a sub-path externally but listening at "/"
//                          internally.
//   - ContainerPort     — per-domain override of the application's
//                          internal_port. NULL falls back to
//                          app.internal_port (the common case).
//   - HTTPS             — issue + serve a TLS cert for this domain
//   - CertificateProvider — "letsencrypt" today; reserved for future
//                            providers (ZeroSSL, Cloudflare Origin CA).
//
// (application_id, host) is unique among live rows; soft-deleted rows
// are excluded from the constraint so re-adding a removed domain works.
type ApplicationDomain struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID       string  `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Host                string  `gorm:"type:varchar(255);not null" json:"host"`
	Path                *string `gorm:"type:varchar(255)" json:"path,omitempty"`
	InternalPath        *string `gorm:"column:internal_path;type:varchar(255)" json:"internal_path,omitempty"`
	StripPath           bool    `gorm:"column:strip_path;not null;default:false" json:"strip_path"`
	ContainerPort       *int    `gorm:"column:container_port;type:int" json:"container_port,omitempty"`
	HTTPS               bool    `gorm:"column:https;not null;default:true" json:"https"`
	CertificateProvider string  `gorm:"column:certificate_provider;type:varchar(32);not null;default:letsencrypt" json:"certificate_provider"`
	CertificateID       *string `gorm:"column:certificate_id;type:char(26)" json:"certificate_id,omitempty"`
}

func (ApplicationDomain) TableName() string { return "docker_application_domains" }
