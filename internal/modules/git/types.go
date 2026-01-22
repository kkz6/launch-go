package git

import (
	"github.com/kkz6/launch-go/internal/modules/git/contracts"
	"github.com/kkz6/launch-go/internal/modules/git/dto"
	"github.com/kkz6/launch-go/internal/modules/git/handlers"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/services"
	gittypes "github.com/kkz6/launch-go/internal/modules/git/types"
)

// Re-export types from subpackages for backward compatibility

// Enums
type GitProviderType = gittypes.GitProviderType

const (
	GitProviderGitHub    = gittypes.GitProviderGitHub
	GitProviderGitLab    = gittypes.GitProviderGitLab
	GitProviderBitbucket = gittypes.GitProviderBitbucket
)

var AllGitProviders = gittypes.AllGitProviders
var ParseGitProviderType = gittypes.ParseGitProviderType
var ParseAccountType = gittypes.ParseAccountType

type AccountType = gittypes.AccountType

const (
	AccountTypeUser         = gittypes.AccountTypeUser
	AccountTypeOrganization = gittypes.AccountTypeOrganization
)

type RepositorySelection = gittypes.RepositorySelection

const (
	RepositorySelectionAll      = gittypes.RepositorySelectionAll
	RepositorySelectionSelected = gittypes.RepositorySelectionSelected
)

type WebhookEventType = gittypes.WebhookEventType

const (
	WebhookEventPush                = gittypes.WebhookEventPush
	WebhookEventPullRequest         = gittypes.WebhookEventPullRequest
	WebhookEventInstallation        = gittypes.WebhookEventInstallation
	WebhookEventInstallationRepos   = gittypes.WebhookEventInstallationRepos
	WebhookEventRepositoriesAdded   = gittypes.WebhookEventRepositoriesAdded
	WebhookEventRepositoriesRemoved = gittypes.WebhookEventRepositoriesRemoved
	WebhookEventInstallationCreated = gittypes.WebhookEventInstallationCreated
	WebhookEventInstallationDeleted = gittypes.WebhookEventInstallationDeleted
)

// Models
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
