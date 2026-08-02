package jobs

import (
	"context"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

type statusProbeServiceRepository struct {
	contracts.ServiceRepository
	updated bool
	called  bool
}

func (r *statusProbeServiceRepository) UpdateStatusFromProbe(
	_ context.Context,
	_ string,
	_ types.ServiceStatus,
	_ map[string]any,
) (bool, error) {
	r.called = true
	return r.updated, nil
}

type statusProbeRegistry struct {
	contracts.RepositoryRegistry
	serviceRepository contracts.ServiceRepository
}

func (r *statusProbeRegistry) Service() contracts.ServiceRepository {
	return r.serviceRepository
}

func TestCheckServiceStatusJobHonorsProtectedProbeUpdate(t *testing.T) {
	repository := &statusProbeServiceRepository{updated: false}
	logger := zerolog.New(io.Discard)
	job := &CheckServiceStatusJob{
		Deps: &JobDeps{
			Deps:  &pkgjobs.Deps{Logger: &logger},
			Repos: &statusProbeRegistry{serviceRepository: repository},
		},
	}

	updated, err := job.updateServiceStatus(
		context.Background(),
		"service-php83",
		types.ServiceStatusRunning,
		"active (running)",
		nil,
		"",
	)

	assert.NoError(t, err)
	assert.True(t, repository.called)
	assert.False(t, updated)
}
