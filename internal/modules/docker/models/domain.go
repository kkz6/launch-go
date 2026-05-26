package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationDomain attaches a public hostname to a docker workload.
// Traefik routes requests for this host to the target container.
// Polymorphic by owner — every live row sets EXACTLY one of
// `ApplicationID` or `ComposeID` (enforced at the service layer); the
// other stays NULL. Same shape we use for ApplicationVolume.
//
// The type name stays `ApplicationDomain` (rather than `DockerDomain`)
// to keep the diff small — every reference would move otherwise. The
// table name `docker_application_domains` is similarly preserved.
//
// Field set mirrors dokploy's Domain model so the UI can reuse the
// same dialog shape:
//
//   - Host              — public hostname (api.example.com)
//   - Path              — external URL path Traefik routes from
//     ("/api"). Empty = match all paths.
//   - InternalPath      — path the application expects internally
//     ("/"). When ExternalPath != InternalPath,
//     Traefik rewrites between them.
//   - StripPath         — when true, strip Path before forwarding to
//     the container. Useful for apps mounted at
//     a sub-path externally but listening at "/"
//     internally.
//   - ContainerPort     — per-domain override of the workload's
//     internal_port. For applications: NULL
//     falls back to app.internal_port. For
//     compose: NULL is rejected at the service
//     layer (no fallback — the operator must
//     name the port their service listens on
//     in the YAML).
//   - HTTPS             — issue + serve a TLS cert for this domain
//   - CertificateProvider — "letsencrypt" today; reserved for future
//     providers (ZeroSSL, Cloudflare Origin CA).
//   - ServiceName       — compose-only. Names the YAML service this
//     domain routes to. Resolved at render time
//     to the compose container name
//     `<project>-<compose>-<service>-1` (compose
//     v2's default naming). Operator must keep
//     their YAML service name in sync — same
//     trust model as the rest of the compose
//     surface.
//
// Uniqueness per-owner: (application_id, host) and (compose_id, host)
// are both unique among live rows; soft-deleted rows are excluded
// from the constraint so re-adding a removed domain works.
type ApplicationDomain struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	// One of ApplicationID / ComposeID is set per row. Both are
	// nullable; the service layer rejects writes that set neither or
	// both. Indexes on both columns so per-owner List queries stay
	// cheap.
	ApplicationID       *string `gorm:"column:application_id;type:char(26);index" json:"application_id,omitempty"`
	ComposeID           *string `gorm:"column:compose_id;type:char(26);index" json:"compose_id,omitempty"`
	Host                string  `gorm:"type:varchar(255);not null" json:"host"`
	Path                *string `gorm:"type:varchar(255)" json:"path,omitempty"`
	InternalPath        *string `gorm:"column:internal_path;type:varchar(255)" json:"internal_path,omitempty"`
	StripPath           bool    `gorm:"column:strip_path;not null;default:false" json:"strip_path"`
	ContainerPort       *int    `gorm:"column:container_port;type:int" json:"container_port,omitempty"`
	HTTPS               bool    `gorm:"column:https;not null;default:true" json:"https"`
	CertificateProvider string  `gorm:"column:certificate_provider;type:varchar(32);not null;default:letsencrypt" json:"certificate_provider"`
	CertificateID       *string `gorm:"column:certificate_id;type:char(26)" json:"certificate_id,omitempty"`
	// ServiceName names the compose YAML service this row targets.
	// NULL on application-owned rows; required on compose-owned rows
	// (service layer rejects empty / nil).
	ServiceName *string `gorm:"column:service_name;type:varchar(255)" json:"service_name,omitempty"`
}

func (ApplicationDomain) TableName() string { return "docker_application_domains" }
