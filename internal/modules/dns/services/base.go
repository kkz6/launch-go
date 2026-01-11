package services

import (
	"errors"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/dns/repositories"
)

var (
	ErrProviderNotFound         = errors.New("provider not found")
	ErrDomainNotFound           = errors.New("domain not found")
	ErrRecordNotFound           = errors.New("record not found")
	ErrRecordNotEditable        = errors.New("record cannot be edited")
	ErrRecordNotDeletable       = errors.New("record cannot be deleted")
	ErrProviderHasActiveDomains = errors.New("provider has active domains")
	ErrInvalidCredentials       = errors.New("invalid credentials")
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
