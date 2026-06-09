package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestEngineManageable(t *testing.T) {
	assert.True(t, engineManageable(dockertypes.DatabaseEngineMySQL))
	assert.True(t, engineManageable(dockertypes.DatabaseEngineMariaDB))
	assert.True(t, engineManageable(dockertypes.DatabaseEnginePostgres))
	assert.False(t, engineManageable(dockertypes.DatabaseEngineRedis))
	assert.False(t, engineManageable(dockertypes.DatabaseEngineMongo))
}

func TestDBObjectNamePattern(t *testing.T) {
	for _, ok := range []string{"app", "my_db", "DB1", "a_b_c_123"} {
		assert.True(t, dbObjectNamePattern.MatchString(ok), ok)
	}
	for _, bad := range []string{"", "drop db", "a;b", "a-b", "`x`", "a'b", "a/b", "über"} {
		assert.False(t, dbObjectNamePattern.MatchString(bad), bad)
	}
}

func TestIsSystemDatabase(t *testing.T) {
	assert.True(t, isSystemDatabase(dockertypes.DatabaseEnginePostgres, "template0"))
	assert.True(t, isSystemDatabase(dockertypes.DatabaseEnginePostgres, "postgres"))
	assert.False(t, isSystemDatabase(dockertypes.DatabaseEnginePostgres, "app"))
	assert.True(t, isSystemDatabase(dockertypes.DatabaseEngineMySQL, "information_schema"))
	assert.True(t, isSystemDatabase(dockertypes.DatabaseEngineMySQL, "mysql"))
	assert.False(t, isSystemDatabase(dockertypes.DatabaseEngineMySQL, "app"))
}

func TestShQuote(t *testing.T) {
	assert.Equal(t, `'plain'`, shQuote("plain"))
	assert.Equal(t, `'a'\''b'`, shQuote("a'b"))
}
