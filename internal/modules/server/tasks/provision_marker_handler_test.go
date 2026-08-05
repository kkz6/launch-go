package tasks

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
)

func markerDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}))
	return db
}

func markerServer(t *testing.T, db *gorm.DB) *models.Server {
	t.Helper()
	server := &models.Server{Name: "web-1"}
	server.ID = "srv-1"
	server.TeamID = "team-1"
	require.NoError(t, db.Create(server).Error)
	return server
}

func newMarkerHandler(t *testing.T, db *gorm.DB, broadcaster *recoveryBroadcaster) *ProvisionMarkerHandler {
	t.Helper()
	return NewProvisionMarkerHandler(ProvisionMarkerHandlerConfig{
		DB:          db,
		Broadcaster: broadcaster,
		Logger:      recoveryLogger(),
		ServerID:    "srv-1",
		TeamID:      "team-1",
	})
}

func eventNames(b *recoveryBroadcaster) []string {
	names := make([]string, 0, len(b.events))
	for _, event := range b.events {
		names = append(names, event.event)
	}
	return names
}

// detectedFacts reads the detected_* columns directly. Scanning the whole
// Server model back out of sqlite fails on detected_at, whose timestamptz
// column round-trips as text — a driver quirk, not product behaviour.
type detectedFacts struct {
	DetectedOSID              *string
	DetectedOSVersion         *string
	DetectedOSVersionCodename *string
	DetectedArch              *string
	DetectedKernel            *string
	DetectedAt                *string
}

func readDetectedFacts(t *testing.T, db *gorm.DB) detectedFacts {
	t.Helper()
	var facts detectedFacts
	require.NoError(t, db.Model(&models.Server{}).
		Select("detected_os_id", "detected_os_version", "detected_os_version_codename",
			"detected_arch", "detected_kernel", "detected_at").
		Where("id = ?", "srv-1").
		Scan(&facts).Error)
	return facts
}

// =============================================================================
// OnMarker dispatch
// =============================================================================

func TestOnMarkerDispatchesByType(t *testing.T) {
	tests := []struct {
		markerType string
		value      string
		wantEvent  string
	}{
		{markerType: markers.Progress, value: "40", wantEvent: "server.provision_progress"},
		{markerType: markers.StepCompleted, value: "configure_swap", wantEvent: "server.provision_step"},
		{markerType: markers.SoftwareInstalled, value: "redis", wantEvent: "server.software_installed"},
		{markerType: markers.Status, value: "Installing Redis", wantEvent: "server.provision_status"},
		{markerType: markers.Error, value: "optional step failed", wantEvent: "server.provision_error"},
		{markerType: markers.DetectedOS, value: "ubuntu|24.04|noble|x86_64|6.8.0", wantEvent: "server.detected_os"},
	}

	for _, tc := range tests {
		t.Run(tc.markerType, func(t *testing.T) {
			db := markerDB(t)
			markerServer(t, db)
			broadcaster := &recoveryBroadcaster{}
			handler := newMarkerHandler(t, db, broadcaster)

			err := handler.OnMarker(context.Background(), "task-1", &markers.Marker{
				Type:  tc.markerType,
				Value: tc.value,
			})

			require.NoError(t, err)
			assert.Contains(t, eventNames(broadcaster), tc.wantEvent)
		})
	}
}

// An unrecognised marker is ignored rather than erroring — scripts may emit
// markers a newer backend understands.
func TestOnMarkerIgnoresUnknownTypes(t *testing.T) {
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, markerDB(t), broadcaster)

	err := handler.OnMarker(context.Background(), "task-1", &markers.Marker{
		Type:  "something_new",
		Value: "x",
	})

	require.NoError(t, err)
	assert.Empty(t, broadcaster.events)
}

// =============================================================================
// handleProgress
// =============================================================================

func TestHandleProgressPersistsAndBroadcasts(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.Progress, Value: "40"}))

	var stored models.Server
	require.NoError(t, db.First(&stored, "id = ?", "srv-1").Error)
	assert.Equal(t, 40, stored.Progress)

	assert.Equal(t, []string{"server.provision_progress", "server.updated"}, eventNames(broadcaster))
}

// The script re-emits its current progress periodically; rebroadcasting an
// unchanged value would spam every connected client.
func TestHandleProgressSkipsRepeatedValues(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	marker := &markers.Marker{Type: markers.Progress, Value: "40"}
	require.NoError(t, handler.OnMarker(context.Background(), "task-1", marker))
	require.NoError(t, handler.OnMarker(context.Background(), "task-1", marker))

	assert.Len(t, broadcaster.events, 2, "the repeat should broadcast nothing")
}

// A failed progress write must not abort provisioning — the script keeps
// running on the server regardless.
func TestHandleProgressToleratesDatabaseFailure(t *testing.T) {
	db := markerDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Server{}))
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.Progress, Value: "40"}))
	assert.NotEmpty(t, broadcaster.events)
}

func TestHandleProgressWithoutDatabase(t *testing.T) {
	broadcaster := &recoveryBroadcaster{}
	handler := NewProvisionMarkerHandler(ProvisionMarkerHandlerConfig{
		Broadcaster: broadcaster,
		ServerID:    "srv-1",
		TeamID:      "team-1",
	})

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.Progress, Value: "40"}))
	assert.NotEmpty(t, broadcaster.events)
}

// =============================================================================
// handleStepCompleted
// =============================================================================

func TestHandleStepCompletedAppendsStep(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.StepCompleted, Value: "configure_swap"}))

	var stored models.Server
	require.NoError(t, db.First(&stored, "id = ?", "srv-1").Error)
	require.NotNil(t, stored.ProgressStep)
	assert.Equal(t, "configure_swap", *stored.ProgressStep)
	assert.Contains(t, stored.CompletedProvisionSteps, "configure_swap")

	assert.Equal(t, []string{"server.provision_step", "server.updated"}, eventNames(broadcaster))
}

// Recovery can replay markers already seen before a worker restart, so the
// same step must not be recorded twice.
func TestHandleStepCompletedIsIdempotent(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	handler := newMarkerHandler(t, db, &recoveryBroadcaster{})

	marker := &markers.Marker{Type: markers.StepCompleted, Value: "configure_swap"}
	require.NoError(t, handler.OnMarker(context.Background(), "task-1", marker))
	require.NoError(t, handler.OnMarker(context.Background(), "task-1", marker))

	var stored models.Server
	require.NoError(t, db.First(&stored, "id = ?", "srv-1").Error)
	assert.Len(t, stored.CompletedProvisionSteps, 1)
}

func TestHandleStepCompletedAccumulatesDistinctSteps(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	handler := newMarkerHandler(t, db, &recoveryBroadcaster{})

	for _, step := range []string{"detect_os", "configure_swap", "configure_firewall"} {
		require.NoError(t, handler.OnMarker(context.Background(), "task-1",
			&markers.Marker{Type: markers.StepCompleted, Value: step}))
	}

	var stored models.Server
	require.NoError(t, db.First(&stored, "id = ?", "srv-1").Error)
	assert.Equal(t, []string{"detect_os", "configure_swap", "configure_firewall"},
		[]string(stored.CompletedProvisionSteps))
}

// A step marker for a server row that isn't there still broadcasts, so the UI
// isn't left frozen.
func TestHandleStepCompletedWithMissingServer(t *testing.T) {
	db := markerDB(t)
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.StepCompleted, Value: "configure_swap"}))
	assert.NotEmpty(t, broadcaster.events)
}

func TestHandleStepCompletedWithoutDatabase(t *testing.T) {
	broadcaster := &recoveryBroadcaster{}
	handler := NewProvisionMarkerHandler(ProvisionMarkerHandlerConfig{
		Broadcaster: broadcaster,
		ServerID:    "srv-1",
		TeamID:      "team-1",
	})

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.StepCompleted, Value: "configure_swap"}))
	assert.NotEmpty(t, broadcaster.events)
}

// =============================================================================
// handleSoftwareInstalled
// =============================================================================

func TestHandleSoftwareInstalledMarksServiceInstalled(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)

	service := &models.InstalledService{
		Software: string(types.SoftwareRedis),
		Status:   types.ServiceStatusInstalling,
	}
	service.ID = "svc-1"
	service.ServerID = "srv-1"
	require.NoError(t, db.Create(service).Error)

	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.SoftwareInstalled, Value: string(types.SoftwareRedis)}))

	var stored models.InstalledService
	require.NoError(t, db.First(&stored, "id = ?", "svc-1").Error)
	assert.Equal(t, types.ServiceStatusInstalled, stored.Status)

	assert.Equal(t, []string{"server.software_installed", "server.updated"}, eventNames(broadcaster))
}

func TestHandleSoftwareInstalledToleratesDatabaseFailure(t *testing.T) {
	db := markerDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.InstalledService{}))
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.SoftwareInstalled, Value: "redis"}))
	assert.NotEmpty(t, broadcaster.events)
}

func TestHandleSoftwareInstalledWithoutDatabase(t *testing.T) {
	broadcaster := &recoveryBroadcaster{}
	handler := NewProvisionMarkerHandler(ProvisionMarkerHandlerConfig{
		Broadcaster: broadcaster,
		ServerID:    "srv-1",
		TeamID:      "team-1",
	})

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.SoftwareInstalled, Value: "redis"}))
	assert.NotEmpty(t, broadcaster.events)
}

// =============================================================================
// handleStatus / handleError
// =============================================================================

// Status is broadcast only — it's a transient label, not persisted state.
func TestHandleStatus(t *testing.T) {
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, markerDB(t), broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.Status, Value: "Installing Redis"}))

	require.Len(t, broadcaster.events, 1)
	assert.Equal(t, "server.provision_status", broadcaster.events[0].event)

	data, ok := broadcaster.events[0].data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Installing Redis", data["message"])
}

func TestHandleError(t *testing.T) {
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, markerDB(t), broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.Error, Value: "optional step failed"}))

	require.Len(t, broadcaster.events, 1)
	assert.Equal(t, "server.provision_error", broadcaster.events[0].event)

	data, ok := broadcaster.events[0].data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "optional step failed", data["message"])
	assert.Equal(t, false, data["fatal"], "error markers are advisory, the script keeps going")
}

// =============================================================================
// handleDetectedOS
// =============================================================================

func TestHandleDetectedOSPersistsFacts(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	broadcaster := &recoveryBroadcaster{}
	handler := newMarkerHandler(t, db, broadcaster)

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.DetectedOS, Value: "ubuntu|24.04|noble|x86_64|6.8.0-31-generic"}))

	facts := readDetectedFacts(t, db)
	require.NotNil(t, facts.DetectedOSID)
	assert.Equal(t, "ubuntu", *facts.DetectedOSID)
	require.NotNil(t, facts.DetectedOSVersion)
	assert.Equal(t, "24.04", *facts.DetectedOSVersion)
	require.NotNil(t, facts.DetectedArch)
	assert.Equal(t, "x86_64", *facts.DetectedArch)
	require.NotNil(t, facts.DetectedAt)

	assert.Equal(t, []string{"server.detected_os", "server.updated"}, eventNames(broadcaster))
}

// The script emits empty fields for facts it couldn't determine; those become
// NULL rather than empty strings so IfNotNil checks downstream still work.
func TestHandleDetectedOSStoresNilForMissingFacts(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	handler := newMarkerHandler(t, db, &recoveryBroadcaster{})

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.DetectedOS, Value: "debian||||"}))

	facts := readDetectedFacts(t, db)
	require.NotNil(t, facts.DetectedOSID)
	assert.Equal(t, "debian", *facts.DetectedOSID)
	assert.Nil(t, facts.DetectedOSVersion)
	assert.Nil(t, facts.DetectedOSVersionCodename)
	assert.Nil(t, facts.DetectedArch)
	assert.Nil(t, facts.DetectedKernel)
}

// A short value means the script produced fewer fields than expected; the
// missing ones are padded rather than panicking on a slice index.
func TestHandleDetectedOSPadsShortValues(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	handler := newMarkerHandler(t, db, &recoveryBroadcaster{})

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.DetectedOS, Value: "ubuntu"}))

	facts := readDetectedFacts(t, db)
	require.NotNil(t, facts.DetectedOSID)
	assert.Equal(t, "ubuntu", *facts.DetectedOSID)
	assert.Nil(t, facts.DetectedKernel)
}

// Unlike the other handlers this one returns the error, because losing the OS
// facts changes which packages later steps install.
func TestHandleDetectedOSReturnsDatabaseError(t *testing.T) {
	db := markerDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Server{}))
	handler := newMarkerHandler(t, db, &recoveryBroadcaster{})

	err := handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.DetectedOS, Value: "ubuntu|24.04|noble|x86_64|6.8.0"})
	assert.Error(t, err)
}

func TestHandleDetectedOSWithoutDatabase(t *testing.T) {
	broadcaster := &recoveryBroadcaster{}
	handler := NewProvisionMarkerHandler(ProvisionMarkerHandlerConfig{
		Broadcaster: broadcaster,
		ServerID:    "srv-1",
		TeamID:      "team-1",
	})

	require.NoError(t, handler.OnMarker(context.Background(), "task-1",
		&markers.Marker{Type: markers.DetectedOS, Value: "ubuntu|24.04|noble|x86_64|6.8.0"}))
	assert.NotEmpty(t, broadcaster.events)
}

// =============================================================================
// stringOrNil / broadcast
// =============================================================================

func TestStringOrNil(t *testing.T) {
	assert.Nil(t, stringOrNil(""))
	assert.Nil(t, stringOrNil("   "))

	value := stringOrNil("  ubuntu  ")
	require.NotNil(t, value)
	assert.Equal(t, "ubuntu", *value, "surrounding whitespace is trimmed")
}

func TestBroadcastWithoutBroadcaster(t *testing.T) {
	handler := NewProvisionMarkerHandler(ProvisionMarkerHandlerConfig{
		Logger:   recoveryLogger(),
		ServerID: "srv-1",
		TeamID:   "team-1",
	})

	assert.NotPanics(t, func() {
		handler.broadcast("server.updated", map[string]interface{}{"id": "srv-1"})
	})
}

// Every handler has to work with a nil logger — the recoverer builds one
// without a logger in some paths.
func TestMarkerHandlerWithoutLogger(t *testing.T) {
	db := markerDB(t)
	markerServer(t, db)
	broadcaster := &recoveryBroadcaster{}

	handler := NewProvisionMarkerHandler(ProvisionMarkerHandlerConfig{
		DB:          db,
		Broadcaster: broadcaster,
		ServerID:    "srv-1",
		TeamID:      "team-1",
	})

	for _, marker := range []*markers.Marker{
		{Type: markers.Progress, Value: "10"},
		{Type: markers.StepCompleted, Value: "detect_os"},
		{Type: markers.SoftwareInstalled, Value: "redis"},
		{Type: markers.Status, Value: "working"},
		{Type: markers.Error, Value: "non-fatal"},
		{Type: markers.DetectedOS, Value: "ubuntu|24.04|noble|x86_64|6.8.0"},
	} {
		assert.NotPanics(t, func() {
			_ = handler.OnMarker(context.Background(), "task-1", marker)
		}, "marker type %s", marker.Type)
	}
}
