package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"gorm.io/gorm"
)

// DockerDomainRepository handles database operations for docker domains
type DockerDomainRepository struct {
	db *gorm.DB
}

// NewDockerDomainRepository creates a new DockerDomainRepository
func NewDockerDomainRepository(db *gorm.DB) *DockerDomainRepository {
	return &DockerDomainRepository{db: db}
}

// Create inserts a new docker domain record
func (r *DockerDomainRepository) Create(ctx context.Context, domain *models.DockerDomain) error {
	return r.db.WithContext(ctx).Create(domain).Error
}

// FindByID retrieves a docker domain by ID
func (r *DockerDomainRepository) FindByID(ctx context.Context, id string) (*models.DockerDomain, error) {
	var domain models.DockerDomain

	err := r.db.WithContext(ctx).First(&domain, "id = ?", id).Error

	return &domain, err
}

// FindByServiceID retrieves all domains for a docker service
func (r *DockerDomainRepository) FindByServiceID(ctx context.Context, serviceID string) ([]models.DockerDomain, error) {
	var domains []models.DockerDomain

	err := r.db.WithContext(ctx).Where("docker_service_id = ?", serviceID).Find(&domains).Error

	return domains, err
}

// Update saves changes to a docker domain
func (r *DockerDomainRepository) Update(ctx context.Context, domain *models.DockerDomain) error {
	return r.db.WithContext(ctx).Save(domain).Error
}

// Delete removes a docker domain by ID
func (r *DockerDomainRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.DockerDomain{}, "id = ?", id).Error
}
