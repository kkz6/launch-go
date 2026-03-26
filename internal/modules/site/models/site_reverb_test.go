package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnabledFeature_ReverbFields_JSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	port := 6001
	daemonID := "daemon-456"

	feature := EnabledFeature{
		Name:       "reverb",
		DaemonID:   &daemonID,
		ReverbPort: &port,
		EnabledAt:  &now,
	}

	data, err := json.Marshal(feature)
	require.NoError(t, err)

	var decoded EnabledFeature
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "reverb", decoded.Name)
	require.NotNil(t, decoded.ReverbPort)
	assert.Equal(t, 6001, *decoded.ReverbPort)
	require.NotNil(t, decoded.DaemonID)
	assert.Equal(t, "daemon-456", *decoded.DaemonID)
	assert.Nil(t, decoded.OctanePort)
	assert.Nil(t, decoded.OctaneServer)
	assert.Nil(t, decoded.CronID)
}

func TestEnabledFeature_ReverbFields_OmitEmpty(t *testing.T) {
	feature := EnabledFeature{
		Name: "queue",
	}

	data, err := json.Marshal(feature)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "reverb_port")
	assert.NotContains(t, jsonStr, "octane_port")
	assert.NotContains(t, jsonStr, "octane_server")
}

func TestEnabledFeaturesSlice_ScanValue_WithReverb(t *testing.T) {
	port := 6042

	original := EnabledFeaturesSlice{
		{Name: "queue", QueueID: strPtr("q1")},
		{Name: "reverb", DaemonID: strPtr("d2"), ReverbPort: &port},
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
	assert.Nil(t, scanned[0].ReverbPort)

	assert.Equal(t, "reverb", scanned[1].Name)
	require.NotNil(t, scanned[1].ReverbPort)
	assert.Equal(t, 6042, *scanned[1].ReverbPort)
}

func TestEnabledFeaturesSlice_Scan_StringInput_WithReverb(t *testing.T) {
	input := `[{"name":"reverb","reverb_port":6005,"daemon_id":"d99"}]`
	var scanned EnabledFeaturesSlice
	err := scanned.Scan(input)
	require.NoError(t, err)
	require.Len(t, scanned, 1)
	assert.Equal(t, "reverb", scanned[0].Name)
	require.NotNil(t, scanned[0].ReverbPort)
	assert.Equal(t, 6005, *scanned[0].ReverbPort)
	require.NotNil(t, scanned[0].DaemonID)
	assert.Equal(t, "d99", *scanned[0].DaemonID)
}

func TestSite_GetReverbPort_Enabled(t *testing.T) {
	port := 6042
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
			{Name: "reverb", ReverbPort: &port},
		},
	}

	result := site.GetReverbPort()
	require.NotNil(t, result)
	assert.Equal(t, 6042, *result)
}

func TestSite_GetReverbPort_NotEnabled(t *testing.T) {
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
		},
	}

	result := site.GetReverbPort()
	assert.Nil(t, result)
}

func TestSite_GetReverbPort_EmptyFeatures(t *testing.T) {
	site := &Site{}
	result := site.GetReverbPort()
	assert.Nil(t, result)
}

func TestSite_GetReverbPort_NilReverbPort(t *testing.T) {
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "reverb", DaemonID: strPtr("d1")},
		},
	}

	result := site.GetReverbPort()
	assert.Nil(t, result)
}

func TestSite_AddEnabledFeature_Reverb(t *testing.T) {
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
		},
	}

	port := 6001
	now := time.Now()
	site.AddEnabledFeature(EnabledFeature{
		Name:       "reverb",
		DaemonID:   strPtr("d2"),
		ReverbPort: &port,
		EnabledAt:  &now,
	})

	assert.Len(t, site.EnabledFeatures, 2)
	assert.True(t, site.HasEnabledFeature("reverb"))
	assert.True(t, site.HasEnabledFeature("queue"))

	feature := site.GetEnabledFeature("reverb")
	require.NotNil(t, feature)
	require.NotNil(t, feature.ReverbPort)
	assert.Equal(t, 6001, *feature.ReverbPort)
}

func TestSite_AddEnabledFeature_Reverb_ReplacesExisting(t *testing.T) {
	port1 := 6001
	port2 := 6002
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "reverb", ReverbPort: &port1},
		},
	}

	site.AddEnabledFeature(EnabledFeature{
		Name:       "reverb",
		ReverbPort: &port2,
	})

	assert.Len(t, site.EnabledFeatures, 1)
	feature := site.GetEnabledFeature("reverb")
	require.NotNil(t, feature)
	require.NotNil(t, feature.ReverbPort)
	assert.Equal(t, 6002, *feature.ReverbPort)
}

func TestSite_RemoveEnabledFeature_Reverb(t *testing.T) {
	port := 6001
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "queue", QueueID: strPtr("q1")},
			{Name: "reverb", ReverbPort: &port},
		},
	}

	site.RemoveEnabledFeature("reverb")

	assert.Len(t, site.EnabledFeatures, 1)
	assert.False(t, site.HasEnabledFeature("reverb"))
	assert.True(t, site.HasEnabledFeature("queue"))
	assert.Nil(t, site.GetReverbPort())
}

func TestEnabledFeature_ReverbAndOctane_Coexist(t *testing.T) {
	octanePort := 8000
	reverbPort := 6001

	features := EnabledFeaturesSlice{
		{Name: "octane", OctanePort: &octanePort, OctaneServer: strPtr("frankenphp")},
		{Name: "reverb", ReverbPort: &reverbPort, DaemonID: strPtr("d1")},
	}

	data, err := json.Marshal(features)
	require.NoError(t, err)

	var decoded EnabledFeaturesSlice
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Len(t, decoded, 2)

	assert.Equal(t, "octane", decoded[0].Name)
	require.NotNil(t, decoded[0].OctanePort)
	assert.Equal(t, 8000, *decoded[0].OctanePort)
	assert.Nil(t, decoded[0].ReverbPort)

	assert.Equal(t, "reverb", decoded[1].Name)
	require.NotNil(t, decoded[1].ReverbPort)
	assert.Equal(t, 6001, *decoded[1].ReverbPort)
	assert.Nil(t, decoded[1].OctanePort)
}

func TestSite_GetReverbPort_WithOctaneAlsoEnabled(t *testing.T) {
	octanePort := 8000
	reverbPort := 6005
	site := &Site{
		EnabledFeatures: EnabledFeaturesSlice{
			{Name: "octane", OctanePort: &octanePort, OctaneServer: strPtr("swoole")},
			{Name: "reverb", ReverbPort: &reverbPort},
		},
	}

	result := site.GetReverbPort()
	require.NotNil(t, result)
	assert.Equal(t, 6005, *result)

	octResult := site.GetOctanePort()
	require.NotNil(t, octResult)
	assert.Equal(t, 8000, *octResult)
}
