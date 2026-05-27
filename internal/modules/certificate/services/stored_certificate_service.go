package services

import (
	"github.com/kkz6/launch-go/internal/modules/certificate/repositories"
)

// StoredCertificateService is the service-layer entry point for the
// certificate module. Phase 1 is just the wiring shell; Phase 2
// (Task 2.2+) adds Create / Update / Delete / Usages / fingerprint
// dedupe / cascading-fanout logic.
type StoredCertificateService struct {
	repos *repositories.Registry
}

func NewStoredCertificateService(repos *repositories.Registry) *StoredCertificateService {
	return &StoredCertificateService{repos: repos}
}
