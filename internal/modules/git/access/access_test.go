package access

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCanonicalRepositoryRejectsCredentialAndForeignURLs(t *testing.T) {
	for _, input := range []string{"https://user:token@github.com/org/repo.git", "https://evil.test/org/repo.git", "https://github.com/org/repo?token=value", "https://github.com/org%2Frepo", "https://github.com/org/../repo", "org/repo;echo", "org/$(echo)", "git@evil.test:org/repo", "https://github.com:443/org/repo"} {
		_, err := CanonicalRepository(input)
		require.Error(t, err, input)
	}
	for _, input := range []string{"org/repo", "https://github.com/org/repo.git", "git@github.com:org/repo.git"} {
		name, err := CanonicalRepository(input)
		require.NoError(t, err)
		require.Equal(t, "org/repo", name)
	}
}
func TestSnapshotRequiresEveryGeneration(t *testing.T) {
	_, err := SnapshotFromMap(map[string]any{"git_access": map[string]any{"connection_id": "a", "repository_id": "1", "connection_generation": 1, "grant_version": 1}})
	require.Error(t, err)
	snapshot := Snapshot{ConnectionID: "connection", ConnectionGeneration: 2, RepositoryID: "7", GrantVersion: 3, AuthorizationID: "authorization", AuthorizationGeneration: 4, InstallationGeneration: 5}
	config := map[string]any{}
	StoreSnapshot(config, snapshot)
	decoded, err := SnapshotFromMap(config)
	require.NoError(t, err)
	require.Equal(t, snapshot, *decoded)
	decoded, err = SnapshotFromMap(nil)
	require.NoError(t, err)
	require.Nil(t, decoded)
}
