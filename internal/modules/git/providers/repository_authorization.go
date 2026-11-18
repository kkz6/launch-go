package providers

import (
	"context"
	"github.com/kkz6/launch-go/internal/modules/git/internal/capability"
)

var ErrRepositoryAuthorizationRequired = capability.ErrRequired

// WithoutRepositoryAuthorization is for verifying the sponsor with a user token.
// It cannot mint installation credentials, which require a resolved capability.
func WithoutRepositoryAuthorization(ctx context.Context) context.Context {
	return capability.With(ctx, nil)
}
func repositoryAuthorization(ctx context.Context) (capability.Authorization, error) {
	return capability.From(ctx)
}
