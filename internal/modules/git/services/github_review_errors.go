package services

import (
	"errors"
	"net/http"

	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/pkg/apperror"
)

// Keep unavailable upstream services distinct from invalid local review state.
// Raw upstream errors can include URLs; diagnostics deliberately record only
// the fixed stage name and safe classification fields.
func (s *SourceControlService) githubReviewError(stage string, err error) error {
	if errors.Is(err, access.ErrAccessDenied) || errors.Is(err, access.ErrStaleAccess) || errors.Is(err, errAttempt) {
		return err
	}
	status := 0
	result := apperror.ErrServiceUnavailable.WithCode("git.github_unavailable").
		WithMessage("GitHub could not be reached or verified. Please retry loading repositories.")
	var upstream *providers.GitHubUserAccessError
	if errors.As(err, &upstream) {
		status = upstream.StatusCode
		if status == http.StatusUnavailableForLegalReasons {
			result = apperror.ErrForbidden.WithCode("git.github_access_restricted").
				WithMessage("GitHub has restricted access to this resource. Check its status on GitHub before trying again.")
		} else if upstream.AuthorizationInvalid {
			result = apperror.ErrForbidden.WithCode("git.github_reauthorization_required").
				WithMessage("GitHub authorization is no longer valid. Reconnect GitHub to continue.")
		} else if !upstream.Retryable && (status == http.StatusForbidden || status == http.StatusNotFound) {
			return access.ErrAccessDenied
		}
	}
	if errors.Is(err, providers.ErrPermissionDenied) || errors.Is(err, providers.ErrInstallationNotFound) {
		return access.ErrAccessDenied
	}
	if s.Logger != nil {
		s.Logger.Warn().Str("stage", stage).Int("github_status", status).
			Bool("retryable", result.HTTPStatus == http.StatusServiceUnavailable).
			Msg("GitHub repository review could not be completed")
	}
	return result
}
