package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestEnabledFeature_OctaneFields_JSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	port := 8000
	server := "frankenphp"
	queueID := "queue-123"

	feature := EnabledFeature{
		Name:         "octane",
		QueueID:      &queueID,
		OctanePort:   &port,
		OctaneServer: &server,
		EnabledAt:    &now,
	}

	data, err := json.Marshal(feature)
	require.NoError(t, err)

	var decoded EnabledFeature
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "octane", decoded.Name)
	require.NotNil(t, decoded.OctanePort)
	assert.Equal(t, 8000, *decoded.OctanePort)
	require.NotNil(t, decoded.OctaneServer)
	assert.Equal(t, "frankenphp", *decoded.OctaneServer)
	require.NotNil(t, decoded.QueueID)
	assert.Equal(t, "queue-123", *decoded.QueueID)
	assert.Nil(t, decoded.CronID)
}

func TestEnabledFeature_OctaneFields_OmitEmpty(t *testing.T) {
	feature := EnabledFeature{
		Name: "queue",
	}

	data, err := json.Marshal(feature)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "octane_port")
	assert.NotContains(t, jsonStr, "octane_server")
	assert.NotContains(t, jsonStr, "queue_id")
	assert.NotContains(t, jsonStr, "cron_id")
}

func TestEnabledFeaturesSlice_ScanValue_WithOctane(t *testing.T) {
	port := 8001
	server := "swoole"

	original := EnabledFeaturesSlice{
		{Name: "queue", QueueID: strPtr("q1")},
		{Name: "octane", QueueID: strPtr("q2"), OctanePort: &port, OctaneServer: &server},
	}

	// Value (serialize for DB)
	val, err := original.Value()
	require.NoError(t, err)
	require.NotNil(t, val)

	// Scan (deserialize from DB)
	var scanned EnabledFeaturesSlice
	err = scanned.Scan(val)
	require.NoError(t, err)
	require.Len(t, scanned, 2)

	assert.Equal(t, "queue", scanned[0].Name)
	assert.Nil(t, scanned[0].OctanePort)

	assert.Equal(t, "octane", scanned[1].Name)
	require.NotNil(t, scanned[1].OctanePort)
	assert.Equal(t, 8001, *scanned[1].OctanePort)
	require.NotNil(t, scanned[1].OctaneServer)
	assert.Equal(t, "swoole", *scanned[1].OctaneServer)
}

func TestEnabledFeaturesSlice_Scan_NilValue(t *testing.T) {
	var scanned EnabledFeaturesSlice
	err := scanned.Scan(nil)
	require.NoError(t, err)
	assert.Nil(t, scanned)
}

func TestEnabledFeaturesSlice_Scan_NullString(t *testing.T) {
	var scanned EnabledFeaturesSlice
	err := scanned.Scan([]byte("null"))
	require.NoError(t, err)
	assert.Nil(t, scanned)
}

func TestEnabledFeaturesSlice_Scan_StringInput(t *testing.T) {
	input := `[{"name":"octane","octane_port":8000,"octane_server":"roadrunner"}]`
	var scanned EnabledFeaturesSlice
	err := scanned.Scan(input)
	require.NoError(t, err)
	require.Len(t, scanned, 1)
	assert.Equal(t, "octane", scanned[0].Name)
	require.NotNil(t, scanned[0].OctanePort)
	assert.Equal(t, 8000, *scanned[0].OctanePort)
	require.NotNil(t, scanned[0].OctaneServer)
	assert.Equal(t, "roadrunner", *scanned[0].OctaneServer)
}

func TestSite_GetOctanePort_Enabled(t *testing.T) {
	port := 8042
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
			{Name: "octane", OctanePort: &port, OctaneServer: strPtr("frankenphp")},
		},
	}

	result := site.GetOctanePort()
	require.NotNil(t, result)
	assert.Equal(t, 8042, *result)
}

func TestSite_GetOctanePort_NotEnabled(t *testing.T) {
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
		},
	}

	result := site.GetOctanePort()
	assert.Nil(t, result)
}

func TestSite_GetOctanePort_EmptyFeatures(t *testing.T) {
	site := &Site{}
	result := site.GetOctanePort()
	assert.Nil(t, result)
}

func TestSite_GetOctanePort_NilOctanePort(t *testing.T) {
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "octane", OctaneServer: strPtr("swoole")},
		},
	}

	result := site.GetOctanePort()
	assert.Nil(t, result)
}

func TestSite_AddEnabledFeature_Octane(t *testing.T) {
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
		},
	}

	port := 8000
	server := "frankenphp"
	now := time.Now()
	site.AddEnabledFeature(EnabledFeature{
		Name:         "octane",
		QueueID:      strPtr("q2"),
		OctanePort:   &port,
		OctaneServer: &server,
		EnabledAt:    &now,
	})

	assert.Len(t, site.EnabledFeatures, 2)
	assert.True(t, site.HasEnabledFeature("octane"))
	assert.True(t, site.HasEnabledFeature("queue"))

	feature := site.GetEnabledFeature("octane")
	require.NotNil(t, feature)
	require.NotNil(t, feature.OctanePort)
	assert.Equal(t, 8000, *feature.OctanePort)
	require.NotNil(t, feature.OctaneServer)
	assert.Equal(t, "frankenphp", *feature.OctaneServer)
}

func TestSite_AddEnabledFeature_Octane_ReplacesExisting(t *testing.T) {
	port1 := 8000
	port2 := 8001
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "octane", OctanePort: &port1, OctaneServer: strPtr("frankenphp")},
		},
	}

	site.AddEnabledFeature(EnabledFeature{
		Name:         "octane",
		OctanePort:   &port2,
		OctaneServer: strPtr("swoole"),
	})

	assert.Len(t, site.EnabledFeatures, 1)
	feature := site.GetEnabledFeature("octane")
	require.NotNil(t, feature)
	require.NotNil(t, feature.OctanePort)
	assert.Equal(t, 8001, *feature.OctanePort)
	require.NotNil(t, feature.OctaneServer)
	assert.Equal(t, "swoole", *feature.OctaneServer)
}

func TestSite_RemoveEnabledFeature_Octane(t *testing.T) {
	port := 8000
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
			{Name: "octane", OctanePort: &port, OctaneServer: strPtr("frankenphp")},
		},
	}

	site.RemoveEnabledFeature("octane")

	assert.Len(t, site.EnabledFeatures, 1)
	assert.False(t, site.HasEnabledFeature("octane"))
	assert.True(t, site.HasEnabledFeature("queue"))
	assert.Nil(t, site.GetOctanePort())
}

func TestEnabledFeature_AllOctaneServerTypes_JSONRoundTrip(t *testing.T) {
	servers := []string{"frankenphp", "swoole", "roadrunner"}

	for _, server := range servers {
		t.Run(server, func(t *testing.T) {
			port := 8000
			s := server
			feature := EnabledFeature{
				Name:         "octane",
				OctanePort:   &port,
				OctaneServer: &s,
			}

			data, err := json.Marshal(feature)
			require.NoError(t, err)

			var decoded EnabledFeature
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			require.NotNil(t, decoded.OctaneServer)
			assert.Equal(t, server, *decoded.OctaneServer)
		})
	}
}
