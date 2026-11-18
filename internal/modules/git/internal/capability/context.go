// Package capability keeps the provider authorization context inside the git
// module. Other modules can obtain authority only from the database resolver.
package capability

import (
	"context"
	"errors"
	"time"
)

var ErrRequired = errors.New("verified repository selection is required")

type Authorization interface {
	TokenScope(context.Context, string) ([]int64, map[string]string, error)
	Validate(context.Context) error
	RecordToken(context.Context, string, time.Time) error
}
type key struct{}

func With(ctx context.Context, authorization Authorization) context.Context {
	return context.WithValue(ctx, key{}, authorization)
}
func From(ctx context.Context) (Authorization, error) {
	authorization, ok := ctx.Value(key{}).(Authorization)
	if !ok || authorization == nil {
		return nil, ErrRequired
	}
	return authorization, nil
}
