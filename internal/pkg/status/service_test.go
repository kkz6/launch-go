package status

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceStatusJSONSerialization(t *testing.T) {
	t.Run("full service status", func(t *testing.T) {
		service := ServiceStatus{
			ID:       "service-123",
			Software: "nginx",
			Name:     "nginx.service",
			Status:   "running",
			IsActive: true,
			Memory:   "256 MB",
			Uptime:   "5d 12h 30m",
			PID:      1234,
		}

		data, err := json.Marshal(service)
		assert.NoError(t, err)

		var decoded ServiceStatus
		err = json.Unmarshal(data, &decoded)
		assert.NoError(t, err)

		assert.Equal(t, service, decoded)
	})

	t.Run("omits empty optional fields", func(t *testing.T) {
		service := ServiceStatus{
			ID:       "service-456",
			Software: "php-fpm",
			Name:     "php8.2-fpm.service",
			Status:   "stopped",
			IsActive: false,
		}

		data, err := json.Marshal(service)
		assert.NoError(t, err)

		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "memory")
		assert.NotContains(t, jsonStr, "uptime")
		// PID with omitempty and zero value will be omitted
		assert.NotContains(t, jsonStr, "pid")
	})

	t.Run("deserialize from JSON", func(t *testing.T) {
		jsonData := `{
			"id": "svc-789",
			"software": "mysql",
			"name": "mysql.service",
			"status": "active",
			"is_active": true,
			"memory": "512 MB",
			"uptime": "3h 45m",
			"pid": 5678
		}`

		var service ServiceStatus
		err := json.Unmarshal([]byte(jsonData), &service)
		assert.NoError(t, err)

		assert.Equal(t, "svc-789", service.ID)
		assert.Equal(t, "mysql", service.Software)
		assert.Equal(t, "mysql.service", service.Name)
		assert.Equal(t, "active", service.Status)
		assert.True(t, service.IsActive)
		assert.Equal(t, "512 MB", service.Memory)
		assert.Equal(t, "3h 45m", service.Uptime)
		assert.Equal(t, 5678, service.PID)
	})
}
