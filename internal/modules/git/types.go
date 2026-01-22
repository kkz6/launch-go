package git

import (
	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/enums"
	"github.com/kkz6/launch-go/internal/modules/git/handlers"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/services"
)

// Re-export types from subpackages for backward compatibility

// Enums
type GitProviderType = enums.GitProviderType

const (
	GitProviderGitHub    = enums.GitProviderGitHub
	GitProviderGitLab    = enums.GitProviderGitLab
	GitProviderBitbucket = enums.GitProviderBitbucket
)

var AllGitProviders = enums.AllGitProviders
var ParseGitProviderType = enums.ParseGitProviderType
var ParseAccountType = enums.ParseAccountType

type AccountType = enums.AccountType

const (
	AccountTypeUser         = enums.AccountTypeUser
	AccountTypeOrganization = enums.AccountTypeOrganization
)

type RepositorySelection = enums.RepositorySelection

const (
	RepositorySelectionAll      = enums.RepositorySelectionAll
	RepositorySelectionSelected = enums.RepositorySelectionSelected
)

type WebhookEventType = enums.WebhookEventType

const (
	WebhookEventPush                = enums.WebhookEventPush
	WebhookEventPullRequest         = enums.WebhookEventPullRequest
	WebhookEventInstallation        = enums.WebhookEventInstallation
	WebhookEventInstallationRepos   = enums.WebhookEventInstallationRepos
	WebhookEventRepositoriesAdded   = enums.WebhookEventRepositoriesAdded
	WebhookEventRepositoriesRemoved = enums.WebhookEventRepositoriesRemoved
	WebhookEventInstallationCreated = enums.WebhookEventInstallationCreated
	WebhookEventInstallationDeleted = enums.WebhookEventInstallationDeleted
)

// Models
type JSONMap = models.JSONMap
type JSONArray = models.JSONArray
type SourceControl = models.SourceControl
type SourceControlRepository = models.SourceControlRepository

// DTOs - Requests
type ConnectProviderRequest = dto.ConnectProviderRequest
type RefreshRepositoriesRequest = dto.RefreshRepositoriesRequest

// DTOs - Responses
type SourceControlResponse = dto.SourceControlResponse
type RepositoryResponse = dto.RepositoryResponse
type InstallationURLResponse = dto.InstallationURLResponse
type InstallationsResponse = dto.InstallationsResponse
type InstallationSummaryData = dto.InstallationSummaryData

// DTOs - Data
type AppInstallationData = dto.AppInstallationData
type CommitData = dto.CommitData
type RepositoryData = dto.RepositoryData
type WebhookPayload = dto.WebhookPayload

// DTO Functions
var ToSourceControlResponse = dto.ToSourceControlResponse
var ToRepositoryResponse = dto.ToRepositoryResponse
var InstallationSummaryFromSourceControl = dto.InstallationSummaryFromSourceControl
var CommitDataFromGitHubPayload = dto.CommitDataFromGitHubPayload
var CommitDataFromGitLabPayload = dto.CommitDataFromGitLabPayload
var CommitDataFromBitbucketPayload = dto.CommitDataFromBitbucketPayload
var RepositoryDataFromAPIResponse = dto.RepositoryDataFromAPIResponse

// Repository query options
type InstallationQueryOption = contracts.InstallationQueryOption

var WithUserID = contracts.WithUserID
var WithProviderID = contracts.WithProviderID
var RequireInstallationID = contracts.RequireInstallationID

// Services
type Service = services.SourceControlService

// Handlers
type Handler = handlers.SourceControlHandler
type WebhookHandler = handlers.WebhookHandler

// Constructor functions
var NewHandler = handlers.NewSourceControlHandler
var NewWebhookHandler = handlers.NewWebhookHandler
