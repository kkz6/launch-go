package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationDomain attaches a public hostname to a docker application.
// Traefik routes requests for this host to the application's container.
// Each domain is independent — adding/removing one rewrites the Traefik
// dynamic-config file for the app, no redeploy required.
//
// The (application_id, host) pair is unique among live rows; soft-
// deleted rows are excluded from the constraint so re-adding a removed
// domain works.
type ApplicationDomain struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string  `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Host          string  `gorm:"type:varchar(255);not null" json:"host"`
	Path          *string `gorm:"type:varchar(255)" json:"path,omitempty"`
	HTTPS         bool    `gorm:"column:https;not null;default:true" json:"https"`
	CertificateID *string `gorm:"column:certificate_id;type:char(26)" json:"certificate_id,omitempty"`
}

func (ApplicationDomain) TableName() string { return "docker_application_domains" }
