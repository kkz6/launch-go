package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerType_Docker_IsValid(t *testing.T) {
	assert.True(t, ServerTypeDocker.IsValid(), "docker server type must be valid")
	assert.Equal(t, "Docker Application Server", ServerTypeDocker.Label())
}

func TestServerType_Docker_GetFeatures(t *testing.T) {
	got := ServerTypeDocker.GetFeatures()

	// Docker servers expose Services (for managing docker/traefik) and SSL (via Traefik).
	// They deliberately don't expose PHP/composer/database/queue features — those
	// are PHP-stack concepts and don't apply to the docker model.
	want := []ServerFeature{
		ServerFeatureSSLCertificates,
		ServerFeatureServices,
	}
	assert.Equal(t, want, got)

	for _, unsupported := range []ServerFeature{
		ServerFeaturePhpManagement,
		ServerFeatureComposer,
		ServerFeatureDatabaseManagement,
		ServerFeatureQueueWorkers,
		ServerFeatureSites,
	} {
		assert.False(t, ServerTypeDocker.HasFeature(unsupported),
			"docker server should not expose %s", unsupported)
	}
}

func TestServerType_Docker_ProcessManager(t *testing.T) {
	// Docker servers don't use a host-level process manager (containers handle it).
	assert.Equal(t, ProcessManagerNone, ServerTypeDocker.GetProcessManager())
}

func TestServerType_Docker_InAllList(t *testing.T) {
	found := false
	for _, t := range AllServerTypes() {
		if t == ServerTypeDocker {
			found = true
			break
		}
	}
	assert.True(t, found, "ServerTypeDocker must appear in AllServerTypes()")
}

func TestServiceType_Docker_Traefik(t *testing.T) {
	assert.True(t, ServiceTypeDocker.IsValid())
	assert.True(t, ServiceTypeTraefik.IsValid())
	assert.Equal(t, "Docker", ServiceTypeDocker.Label())
	assert.Equal(t, "Traefik", ServiceTypeTraefik.Label())

	// Neither is a database service.
	assert.False(t, ServiceTypeDocker.IsDatabase())
	assert.False(t, ServiceTypeTraefik.IsDatabase())
}
