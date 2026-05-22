package providers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/launch/sshkey"
)

// withStubAWSValidator swaps the package's STS-call shim for a stub that
// records its arguments and returns the supplied error. Returns a cleanup
// function the test must defer.
func withStubAWSValidator(t *testing.T, returnErr error) (callCount *int, gotAccess, gotSecret, gotRegion *string, restore func()) {
	t.Helper()
	originalValidator := validateAWSCredentials

	count := 0
	var access, secret, region string

	validateAWSCredentials = func(_ context.Context, accessKey, secretKey, r string) error {
		count++
		access = accessKey
		secret = secretKey
		region = r
		return returnErr
	}

	return &count, &access, &secret, &region, func() {
		validateAWSCredentials = originalValidator
	}
}

func TestAWSConnect_HappyPath(t *testing.T) {
	count, access, secret, region, restore := withStubAWSValidator(t, nil)
	defer restore()

	p := NewAWSProvider(sshkey.NewGenerator())
	err := p.Connect(context.Background(), map[string]any{
		"access_key": "AKIA-real",
		"secret_key": "supersecret",
		"region":     "eu-west-1",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, *count, "STS validator must be invoked exactly once")
	assert.Equal(t, "AKIA-real", *access)
	assert.Equal(t, "supersecret", *secret)
	assert.Equal(t, "eu-west-1", *region)
}

func TestAWSConnect_DefaultRegionWhenMissing(t *testing.T) {
	count, _, _, region, restore := withStubAWSValidator(t, nil)
	defer restore()

	p := NewAWSProvider(sshkey.NewGenerator())
	err := p.Connect(context.Background(), map[string]any{
		"access_key": "AKIA-real",
		"secret_key": "supersecret",
		// region intentionally omitted
	})

	require.NoError(t, err)
	assert.Equal(t, 1, *count)
	assert.Equal(t, defaultAWSValidationRegion, *region,
		"missing region must fall back to %s for STS validation", defaultAWSValidationRegion)
}

func TestAWSConnect_RejectsBadCredentials(t *testing.T) {
	// Simulate STS responding with an auth error.
	count, _, _, _, restore := withStubAWSValidator(t, errors.New("InvalidClientTokenId"))
	defer restore()

	p := NewAWSProvider(sshkey.NewGenerator())
	err := p.Connect(context.Background(), map[string]any{
		"access_key": "AKIA-bad",
		"secret_key": "wrong",
		"region":     "us-east-1",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCredentials,
		"any STS rejection must surface as ErrInvalidCredentials so the UI returns 400")
	assert.Equal(t, 1, *count)
}

func TestAWSConnect_RejectsMissingAccessKey(t *testing.T) {
	count, _, _, _, restore := withStubAWSValidator(t, nil)
	defer restore()

	p := NewAWSProvider(sshkey.NewGenerator())
	err := p.Connect(context.Background(), map[string]any{
		"secret_key": "supersecret",
	})

	require.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Equal(t, 0, *count,
		"missing creds must short-circuit before contacting STS")
}

func TestAWSConnect_RejectsMissingSecretKey(t *testing.T) {
	count, _, _, _, restore := withStubAWSValidator(t, nil)
	defer restore()

	p := NewAWSProvider(sshkey.NewGenerator())
	err := p.Connect(context.Background(), map[string]any{
		"access_key": "AKIA-something",
	})

	require.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Equal(t, 0, *count, "missing creds must short-circuit before contacting STS")
}
