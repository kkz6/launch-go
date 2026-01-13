package services

import (
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// Service errors with HTTP status codes
var (
	ErrProviderNotFound         = response.ErrNotFound("Provider not found")
	ErrDomainNotFound           = response.ErrNotFound("Domain not found")
	ErrRecordNotFound           = response.ErrNotFound("Record not found")
	ErrRecordNotEditable        = response.ErrBadRequest("Record cannot be edited")
	ErrRecordNotDeletable       = response.ErrBadRequest("Record cannot be deleted")
	ErrProviderHasActiveDomains = response.ErrConflict("Provider has active domains")
	ErrInvalidCredentials       = response.ErrBadRequest("Invalid credentials")
)

// BaseService provides common functionality for services
type BaseService struct {
	domainProviderRepo *repositories.DomainProviderRepository
	domainRepo         *repositories.DomainRepository
	dnsRecordRepo      *repositories.DnsRecordRepository
	logger             *zerolog.Logger
}

// NewBaseService creates a new BaseService instance
func NewBaseService(
	domainProviderRepo *repositories.DomainProviderRepository,
	domainRepo *repositories.DomainRepository,
	dnsRecordRepo *repositories.DnsRecordRepository,
	logger *zerolog.Logger,
) *BaseService {
	return &BaseService{
		domainProviderRepo: domainProviderRepo,
		domainRepo:         domainRepo,
		dnsRecordRepo:      dnsRecordRepo,
		logger:             logger,
	}
}
