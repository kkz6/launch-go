package tasks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetServiceStatusTaskUsesPHPSeriesWhenStoredVersionHasPatch(t *testing.T) {
	t.Parallel()

	task := GetServiceStatusTask("php83", "8.3.6")

	assert.Equal(t, "Check PHP 8.3 Status", task.Name())
	assert.Contains(t, task.Script(), "systemctl status php8.3-fpm")
	assert.NotContains(t, task.Script(), "php8.3.6")
}
