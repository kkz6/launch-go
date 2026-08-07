package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(value string) *string { return &value }

// Hooks arrive already populated from the site type's defaults, so the
// request has to be able to override one, leave one alone, and remove one.
func TestApplyRequestedHook(t *testing.T) {
	tests := []struct {
		name      string
		current   *string
		requested *string
		want      *string
	}{
		{
			// Nothing was asked for, so whatever GetDefaultAttributes put
			// there stands. This is the common path — a create request that
			// never opens the deployment settings section.
			name:      "nil request keeps the type default",
			current:   strPtr("php artisan migrate --force"),
			requested: nil,
			want:      strPtr("php artisan migrate --force"),
		},
		{
			name:      "nil request over no default stays empty",
			current:   nil,
			requested: nil,
			want:      nil,
		},
		{
			name:      "a value replaces the default",
			current:   strPtr("php artisan migrate --force"),
			requested: strPtr("git submodule update --init --recursive"),
			want:      strPtr("git submodule update --init --recursive"),
		},
		{
			name:      "a value applies when there is no default",
			current:   nil,
			requested: strPtr("git submodule update --init --recursive"),
			want:      strPtr("git submodule update --init --recursive"),
		},
		{
			// Without this there would be no way to run a site type whose
			// default hook you do not want.
			name:      "an empty string clears the default",
			current:   strPtr("php artisan migrate --force"),
			requested: strPtr(""),
			want:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.current
			applyRequestedHook(&target, tc.requested)

			if tc.want == nil {
				assert.Nil(t, target)
				return
			}
			require.NotNil(t, target)
			assert.Equal(t, *tc.want, *target)
		})
	}
}

// The request value must be copied rather than aliased, or a later mutation
// of the request would reach through into the persisted site.
func TestApplyRequestedHookDoesNotAliasTheRequest(t *testing.T) {
	requested := "git submodule update --init --recursive"
	var target *string

	applyRequestedHook(&target, &requested)
	require.NotNil(t, target)

	requested = "something else entirely"
	assert.Equal(t, "git submodule update --init --recursive", *target)
}
