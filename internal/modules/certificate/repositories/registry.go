package repositories

import "gorm.io/gorm"

// Registry bundles the certificate module's repositories. Matches the
// `Registry` naming convention used by docker / server / site modules.
type Registry struct {
	StoredCertificates *StoredCertificateRepository
}

func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		StoredCertificates: NewStoredCertificateRepository(db),
	}
}
