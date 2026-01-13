package services

import (
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// Service errors - re-exported from centralized error package
var (
	ErrProviderNotFound         = apperrors.ErrDNSProviderNotFound
	ErrDomainNotFound           = apperrors.ErrDomainNotFound
	ErrRecordNotFound           = apperrors.ErrDNSRecordNotFound
	ErrRecordNotEditable        = apperrors.BadRequest("Record cannot be edited")
	ErrRecordNotDeletable       = apperrors.BadRequest("Record cannot be deleted")
	ErrProviderHasActiveDomains = apperrors.Conflict("Provider has active domains")
	ErrInvalidCredentials       = apperrors.BadRequest("Invalid credentials")
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
