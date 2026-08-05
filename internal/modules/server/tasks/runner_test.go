package tasks

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// =============================================================================
// Mock Types
// =============================================================================

// mockDispatcher implements taskrunner.TaskDispatcher for testing
type mockDispatcher struct {
	runFunc   func(ctx context.Context, task *taskrunner.PendingTask) (*taskrunner.TaskResult, error)
	callCount int32
	lastTask  *taskrunner.PendingTask
}

func (m *mockDispatcher) Run(ctx context.Context, task *taskrunner.PendingTask) (*taskrunner.TaskResult, error) {
	atomic.AddInt32(&m.callCount, 1)
	m.lastTask = task
	if m.runFunc != nil {
		return m.runFunc(ctx, task)
	}
	return &taskrunner.TaskResult{
		ExitCode: 0,
		Output:   "success",
	}, nil
}

// mockTask implements taskrunner.Task for testing
type mockTask struct {
	name    string
	script  string
	timeout time.Duration
}

func (t *mockTask) Name() string                                                  { return t.name }
func (t *mockTask) Script() string                                                { return t.script }
func (t *mockTask) Timeout() time.Duration                                        { return t.timeout }
func (t *mockTask) OnOutput(output string)                                        {}
func (t *mockTask) OnFinished(ctx context.Context, result *taskrunner.TaskResult) {}
func (t *mockTask) OnFailed(ctx context.Context, result *taskrunner.TaskResult)   {}
func (t *mockTask) OnTimeout(ctx context.Context, result *taskrunner.TaskResult)  {}

// mockCallbackTask implements both Task and CallbackPayload
type mockCallbackTask struct {
	*mockTask
	mu              sync.RWMutex
	state           mockCallbackState
	onSuccessCalled bool
	onFailureCalled bool
	onExpiredCalled bool
	lastTaskID      string
	lastExitCode    int
	returnError     error
}

func (t *mockCallbackTask) TypeName() string {
	return "test:callback"
}

func (t *mockCallbackTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.state)
}

func (t *mockCallbackTask) OnSuccess(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onSuccessCalled = true
	t.lastTaskID = taskID
	return t.returnError
}

func (t *mockCallbackTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onFailureCalled = true
	t.lastTaskID = taskID
	t.lastExitCode = exitCode
	return t.returnError
}

func (t *mockCallbackTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onExpiredCalled = true
	t.lastTaskID = taskID
	return t.returnError
}

func (t *mockCallbackTask) successCalled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.onSuccessCalled
}

func (t *mockCallbackTask) failureCalled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.onFailureCalled
}

func (t *mockCallbackTask) expiredCalled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.onExpiredCalled
}

type mockCallbackState struct {
	SiteID       string `json:"site_id"`
	DeploymentID string `json:"deployment_id"`
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestServer() *models.Server {
	ip := "192.168.1.100"
	privateKey := "-----BEGIN OPENSSH PRIVATE KEY-----\ntest-key\n-----END OPENSSH PRIVATE KEY-----"

	server := &models.Server{
		BaseModel: basemodels.BaseModel{
			ID: "test-server-123",
		},
		Name:       "Test Server",
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString(privateKey),
	}
	server.TeamID = "test-team-123"
	server.UserID = "test-user-123"
	return server
}

func createTestTask() *mockTask {
	return &mockTask{
		name:    "Test Task",
		script:  "echo 'hello world'",
		timeout: 30 * time.Second,
	}
}

func createTestCallbackTask() *mockCallbackTask {
	return &mockCallbackTask{
		mockTask: &mockTask{
			name:    "Deploy Task",
			script:  "echo 'deploying...'",
			timeout: 60 * time.Second,
		},
		state: mockCallbackState{
			SiteID:       "site-123",
			DeploymentID: "deploy-456",
		},
	}
}

// =============================================================================
// TaskRunner Builder Tests
// =============================================================================

func TestNewTaskRunner(t *testing.T) {
	server := createTestServer()
	task := createTestTask()

	runner := NewTaskRunner(server, task)

	if runner == nil {
		t.Fatal("Expected runner to be created")
	}
	if runner.server != server {
		t.Error("Expected server to be set")
	}
	if runner.task != task {
		t.Error("Expected task to be set")
	}
}

func TestTaskRunner_WithDispatcher(t *testing.T) {
	server := createTestServer()
	task := createTestTask()
	dispatcher := &mockDispatcher{}

	runner := NewTaskRunner(server, task).
		WithDispatcher(dispatcher)

	if runner.dispatcher != dispatcher {
		t.Error("Expected dispatcher to be set")
	}
}

func TestTaskRunner_AsRoot(t *testing.T) {
	server := createTestServer()
	task := createTestTask()

	runner := NewTaskRunner(server, task).AsRoot()

	if !runner.asRoot {
		t.Error("Expected asRoot to be true")
	}
}

func TestTaskRunner_AsUser(t *testing.T) {
	server := createTestServer()
	task := createTestTask()

	runner := NewTaskRunner(server, task).AsUser("custom-user")

	if runner.username != "custom-user" {
		t.Errorf("Expected username 'custom-user', got '%s'", runner.username)
	}
}

func TestTaskRunner_TrackInDB(t *testing.T) {
	server := createTestServer()
	task := createTestTask()

	runner := NewTaskRunner(server, task).TrackInDB()

	if !runner.trackInDB {
		t.Error("Expected trackInDB to be true")
	}
}

func TestTaskRunner_ThrowOnError(t *testing.T) {
	server := createTestServer()
	task := createTestTask()

	runner := NewTaskRunner(server, task).ThrowOnError()

	if !runner.throwOnError {
		t.Error("Expected throwOnError to be true")
	}
}

func TestTaskRunner_BuilderOptions(t *testing.T) {
	runner := NewTaskRunner(createTestServer(), createTestTask())
	logger := zerolog.New(io.Discard)
	marker := taskrunner.MarkerHandlerFunc(func(
		context.Context,
		string,
		*markers.Marker,
	) error {
		return nil
	})

	returned := runner.
		WithQueue(nil).
		WithLogger(&logger).
		WithBroadcaster(nil).
		WithNotifier(nil).
		WithMarkerHandler(marker).
		WithoutTracking().
		Throw()

	assert.Same(t, runner, returned)
	assert.Same(t, &logger, runner.logger)
	assert.NotNil(t, runner.markerHandler)
	assert.False(t, runner.trackInDB)
	assert.True(t, runner.throwOnError)

	runner.AsUser()
	assert.Equal(t, runner.server.GetUsername(), runner.username)
	runner.AsUser("")
	assert.Equal(t, runner.server.GetUsername(), runner.username)
	runner.AsRoot()
	assert.Empty(t, runner.username)
}

func TestTaskRunner_CompletionBuilders(t *testing.T) {
	runner := NewTaskRunner(createTestServer(), createTestTask()).
		OnComplete("job:finished", map[string]string{"id": "1"}).
		OnFailed("job:failed", map[string]string{"id": "2"}).
		OnTimeout("job:timeout", map[string]string{"id": "3"})

	require.NotNil(t, runner.completionConfig)
	assert.Equal(t, "job:finished", runner.completionConfig.OnFinished.Type)
	assert.Equal(t, "job:failed", runner.completionConfig.OnFailed.Type)
	assert.Equal(t, "job:timeout", runner.completionConfig.OnTimeout.Type)

	invalid := func() {}
	runner.OnComplete("invalid", invalid).
		OnFailed("invalid", invalid).
		OnTimeout("invalid", invalid)
	assert.Equal(t, "job:finished", runner.completionConfig.OnFinished.Type)
	assert.Equal(t, "job:failed", runner.completionConfig.OnFailed.Type)
	assert.Equal(t, "job:timeout", runner.completionConfig.OnTimeout.Type)
}

func TestTaskRunner_RunPersistsSuccessAndFailure(t *testing.T) {
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	for _, test := range []struct {
		name      string
		result    *taskrunner.TaskResult
		execErr   error
		throw     bool
		wantError bool
		wantState string
	}{
		{
			name:      "success",
			result:    &taskrunner.TaskResult{ExitCode: 0, Output: "ok"},
			wantState: string(servertypes.TaskStatusFinished),
		},
		{
			name:      "failed result is returned when throwing",
			result:    &taskrunner.TaskResult{ExitCode: 7, Output: "bad"},
			throw:     true,
			wantError: true,
			wantState: string(servertypes.TaskStatusFailed),
		},
		{
			name:      "dispatcher error is persisted",
			execErr:   assert.AnError,
			throw:     true,
			wantError: true,
			wantState: string(servertypes.TaskStatusFailed),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&models.Task{}))

			dispatcher := &mockDispatcher{runFunc: func(
				context.Context,
				*taskrunner.PendingTask,
			) (*taskrunner.TaskResult, error) {
				return test.result, test.execErr
			}}
			runner := NewTaskRunner(createTestServer(), createTestCallbackTask()).
				WithDB(db).
				WithDispatcher(dispatcher).
				TrackInDB()
			if test.throw {
				runner.ThrowOnError()
			}

			result, runErr := runner.Dispatch(context.Background())
			if test.wantError {
				require.Error(t, runErr)
			} else {
				require.NoError(t, runErr)
			}
			require.NotNil(t, result)
			require.NotNil(t, result.TaskModel)

			var persisted models.Task
			require.NoError(t, db.First(&persisted, "id = ?", result.TaskModel.ID).Error)
			assert.Equal(t, test.wantState, persisted.Status)
			if test.execErr != nil {
				assert.Contains(t, persisted.Output.String(), test.execErr.Error())
			}
		})
	}
}

// =============================================================================
// TaskStatus Tests
// =============================================================================

func TestTaskStatus_Constants(t *testing.T) {
	tests := []struct {
		status   servertypes.TaskStatus
		expected string
	}{
		{servertypes.TaskStatusPending, "pending"},
		{servertypes.TaskStatusRunning, "running"},
		{servertypes.TaskStatusFinished, "finished"},
		{servertypes.TaskStatusFailed, "failed"},
		{servertypes.TaskStatusTimeout, "timeout"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("Expected status '%s', got '%s'", tt.expected, string(tt.status))
		}
	}
}

// =============================================================================
// TaskRunnerResult Tests
// =============================================================================

func TestTaskRunnerResult_IsSuccessful(t *testing.T) {
	tests := []struct {
		name     string
		result   *TaskRunnerResult
		expected bool
	}{
		{
			name: "successful task result",
			result: &TaskRunnerResult{
				TaskResult: &taskrunner.TaskResult{ExitCode: 0},
			},
			expected: true,
		},
		{
			name: "failed task result",
			result: &TaskRunnerResult{
				TaskResult: &taskrunner.TaskResult{ExitCode: 1},
			},
			expected: false,
		},
		{
			name: "successful task model",
			result: &TaskRunnerResult{
				TaskModel: &models.Task{ExitCode: intPtr(0)},
			},
			expected: true,
		},
		{
			name: "failed task model",
			result: &TaskRunnerResult{
				TaskModel: &models.Task{ExitCode: intPtr(1)},
			},
			expected: false,
		},
		{
			name:     "empty result",
			result:   &TaskRunnerResult{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsSuccessful(); got != tt.expected {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTaskRunnerResult_GetOutput(t *testing.T) {
	tests := []struct {
		name     string
		result   *TaskRunnerResult
		expected string
	}{
		{
			name: "output from task result",
			result: &TaskRunnerResult{
				TaskResult: &taskrunner.TaskResult{Output: "task output"},
			},
			expected: "task output",
		},
		{
			name: "output from task model",
			result: &TaskRunnerResult{
				TaskModel: &models.Task{Output: "model output"},
			},
			expected: "model output",
		},
		{
			name:     "empty result",
			result:   &TaskRunnerResult{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.GetOutput(); got != tt.expected {
				t.Errorf("GetOutput() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTaskRunnerResult_GetExitCode(t *testing.T) {
	tests := []struct {
		name     string
		result   *TaskRunnerResult
		expected int
	}{
		{
			name: "exit code from task result",
			result: &TaskRunnerResult{
				TaskResult: &taskrunner.TaskResult{ExitCode: 42},
			},
			expected: 42,
		},
		{
			name: "exit code from task model",
			result: &TaskRunnerResult{
				TaskModel: &models.Task{ExitCode: intPtr(127)},
			},
			expected: 127,
		},
		{
			name:     "empty result",
			result:   &TaskRunnerResult{},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.GetExitCode(); got != tt.expected {
				t.Errorf("GetExitCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Run Method Tests (without DB - basic flow)
// =============================================================================

func TestTaskRunner_Run_NoDispatcher(t *testing.T) {
	server := createTestServer()
	task := createTestTask()

	runner := NewTaskRunner(server, task)

	_, err := runner.Run(context.Background())

	if err == nil {
		t.Fatal("Expected error when no dispatcher set")
	}

	expectedMsg := "no dispatcher set: call WithDispatcher() first"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestTaskRunner_Run_NoServerIP(t *testing.T) {
	server := createTestServer()
	server.PublicIPv4 = nil // Remove IP
	task := createTestTask()
	dispatcher := &mockDispatcher{}

	runner := NewTaskRunner(server, task).
		WithDispatcher(dispatcher)

	_, err := runner.Run(context.Background())

	if err == nil {
		t.Fatal("Expected error when server has no IP")
	}
}

func TestTaskRunner_Run_NoPrivateKey(t *testing.T) {
	server := createTestServer()
	server.PrivateKey = "" // Remove key
	task := createTestTask()
	dispatcher := &mockDispatcher{}

	runner := NewTaskRunner(server, task).
		WithDispatcher(dispatcher)

	_, err := runner.Run(context.Background())

	if err == nil {
		t.Fatal("Expected error when server has no private key")
	}
}

func TestTaskRunner_Run_TracksConnectionPreflightFailure(t *testing.T) {
	if err := serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("set encryption key: %v", err)
	}

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Task{}); err != nil {
		t.Fatalf("migrate task: %v", err)
	}

	server := createTestServer()
	server.PublicIPv4 = nil
	callbackTask := createTestCallbackTask()
	dispatcher := &mockDispatcher{}
	var createdTaskID string

	runner := NewTaskRunner(server, callbackTask).
		WithDB(db).
		WithDispatcher(dispatcher).
		TrackInDB().
		OnTaskCreated(func(taskID string) {
			createdTaskID = taskID
		})

	result, runErr := runner.Run(context.Background())

	if runErr == nil {
		t.Fatal("Expected connection preflight error")
	}
	if result == nil || result.TaskModel == nil {
		t.Fatal("Expected the failed task to be tracked")
	}
	if createdTaskID != result.TaskModel.ID {
		t.Fatalf("OnTaskCreated() ID = %q, want %q", createdTaskID, result.TaskModel.ID)
	}
	if atomic.LoadInt32(&dispatcher.callCount) != 0 {
		t.Fatal("Dispatcher must not run when connection preflight fails")
	}

	var stored models.Task
	if err := db.First(&stored, "id = ?", result.TaskModel.ID).Error; err != nil {
		t.Fatalf("find tracked task: %v", err)
	}
	if stored.Status != string(servertypes.TaskStatusFailed) {
		t.Fatalf("task status = %q, want failed", stored.Status)
	}
	if stored.Output.String() != "server has no public IP address" {
		t.Fatalf("task output = %q", stored.Output.String())
	}
	if !callbackTask.onFailureCalled {
		t.Fatal("Expected failure callback for connection preflight error")
	}
}

func TestTaskRunner_AsyncModesTrackConnectionPreflightFailure(t *testing.T) {
	if err := serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("set encryption key: %v", err)
	}

	tests := []struct {
		name string
		run  func(context.Context, *TaskRunner) (*models.Task, error)
	}{
		{
			name: "async",
			run: func(ctx context.Context, runner *TaskRunner) (*models.Task, error) {
				return runner.RunAsync(ctx)
			},
		},
		{
			name: "background",
			run: func(ctx context.Context, runner *TaskRunner) (*models.Task, error) {
				return runner.RunInBackground(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := gorm.Open(
				sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),
				&gorm.Config{},
			)
			if err != nil {
				t.Fatalf("open database: %v", err)
			}
			if err := db.AutoMigrate(&models.Task{}); err != nil {
				t.Fatalf("migrate task: %v", err)
			}

			server := createTestServer()
			server.PublicIPv4 = nil
			callbackTask := createTestCallbackTask()
			dispatcher := &mockDispatcher{}
			var createdTaskID string

			runner := NewTaskRunner(server, callbackTask).
				WithDB(db).
				WithDispatcher(dispatcher).
				OnTaskCreated(func(taskID string) {
					createdTaskID = taskID
				})

			taskModel, runErr := tt.run(context.Background(), runner)

			if runErr == nil {
				t.Fatal("Expected connection preflight error")
			}
			if taskModel == nil {
				t.Fatal("Expected the failed task to be tracked")
			}
			if createdTaskID != taskModel.ID {
				t.Fatalf("OnTaskCreated() ID = %q, want %q", createdTaskID, taskModel.ID)
			}
			if atomic.LoadInt32(&dispatcher.callCount) != 0 {
				t.Fatal("Dispatcher must not run when connection preflight fails")
			}

			var stored models.Task
			if err := db.First(&stored, "id = ?", taskModel.ID).Error; err != nil {
				t.Fatalf("find tracked task: %v", err)
			}
			if stored.Status != string(servertypes.TaskStatusFailed) {
				t.Fatalf("task status = %q, want failed", stored.Status)
			}
			if stored.Output.String() != "server has no public IP address" {
				t.Fatalf("task output = %q", stored.Output.String())
			}
			if !callbackTask.onFailureCalled {
				t.Fatal("Expected failure callback for connection preflight error")
			}
		})
	}
}

// =============================================================================
// Callback Invocation Tests
// =============================================================================

func TestTaskRunner_invokeTaskCallbacks_OnSuccess(t *testing.T) {
	server := createTestServer()
	callbackTask := createTestCallbackTask()

	taskModel := &models.Task{
		BaseModel: basemodels.BaseModel{ID: "task-success"},
	}

	result := &taskrunner.TaskResult{
		ExitCode: 0,
		Output:   "deployment successful",
	}

	runner := NewTaskRunner(server, callbackTask)

	runner.invokeTaskCallbacks(context.Background(), taskModel, result, nil)

	if !callbackTask.onSuccessCalled {
		t.Error("Expected OnSuccess to be called")
	}
	if callbackTask.onFailureCalled {
		t.Error("Expected OnFailure NOT to be called")
	}
	if callbackTask.onExpiredCalled {
		t.Error("Expected OnExpired NOT to be called")
	}
	if callbackTask.lastTaskID != "task-success" {
		t.Errorf("Expected taskID 'task-success', got '%s'", callbackTask.lastTaskID)
	}
}

func TestTaskRunner_invokeTaskCallbacks_OnFailure(t *testing.T) {
	server := createTestServer()
	callbackTask := createTestCallbackTask()

	taskModel := &models.Task{
		BaseModel: basemodels.BaseModel{ID: "task-failure"},
	}

	result := &taskrunner.TaskResult{
		ExitCode: 1,
		Output:   "deployment failed",
	}

	runner := NewTaskRunner(server, callbackTask)

	runner.invokeTaskCallbacks(context.Background(), taskModel, result, nil)

	if callbackTask.onSuccessCalled {
		t.Error("Expected OnSuccess NOT to be called")
	}
	if !callbackTask.onFailureCalled {
		t.Error("Expected OnFailure to be called")
	}
	if callbackTask.onExpiredCalled {
		t.Error("Expected OnExpired NOT to be called")
	}
	if callbackTask.lastExitCode != 1 {
		t.Errorf("Expected exitCode 1, got %d", callbackTask.lastExitCode)
	}
}

func TestTaskRunner_invokeTaskCallbacks_OnExpired(t *testing.T) {
	server := createTestServer()
	callbackTask := createTestCallbackTask()

	taskModel := &models.Task{
		BaseModel: basemodels.BaseModel{ID: "task-timeout"},
	}

	result := &taskrunner.TaskResult{
		ExitCode: 124, // typical timeout exit code
		TimedOut: true,
		Output:   "task timed out",
	}

	runner := NewTaskRunner(server, callbackTask)

	runner.invokeTaskCallbacks(context.Background(), taskModel, result, nil)

	if callbackTask.onSuccessCalled {
		t.Error("Expected OnSuccess NOT to be called")
	}
	if callbackTask.onFailureCalled {
		t.Error("Expected OnFailure NOT to be called")
	}
	if !callbackTask.onExpiredCalled {
		t.Error("Expected OnExpired to be called")
	}
	if callbackTask.lastTaskID != "task-timeout" {
		t.Errorf("Expected taskID 'task-timeout', got '%s'", callbackTask.lastTaskID)
	}
}

func TestTaskRunner_invokeTaskCallbacks_OnError(t *testing.T) {
	server := createTestServer()
	callbackTask := createTestCallbackTask()

	taskModel := &models.Task{
		BaseModel: basemodels.BaseModel{ID: "task-error"},
	}

	execErr := context.DeadlineExceeded

	runner := NewTaskRunner(server, callbackTask)

	runner.invokeTaskCallbacks(context.Background(), taskModel, nil, execErr)

	if callbackTask.onSuccessCalled {
		t.Error("Expected OnSuccess NOT to be called")
	}
	if !callbackTask.onFailureCalled {
		t.Error("Expected OnFailure to be called for execution error")
	}
	if callbackTask.onExpiredCalled {
		t.Error("Expected OnExpired NOT to be called")
	}
}

func TestTaskRunner_invokeTaskCallbacks_NonCallbackTask(t *testing.T) {
	server := createTestServer()
	task := createTestTask() // Regular task, not CallbackPayload

	taskModel := &models.Task{
		BaseModel: basemodels.BaseModel{ID: "task-regular"},
	}

	result := &taskrunner.TaskResult{ExitCode: 0}

	runner := NewTaskRunner(server, task)

	// Should not panic - gracefully handle non-callback tasks
	runner.invokeTaskCallbacks(context.Background(), taskModel, result, nil)
}

// =============================================================================
// Helper Functions for Tests
// =============================================================================

func intPtr(i int) *int {
	return &i
}

func TestTaskRunner_ForSite(t *testing.T) {
	runner := NewTaskRunner(createTestServer(), createTestTask()).ForSite("site-1")

	if runner.siteID == nil {
		t.Fatal("Expected siteID to be set")
	}
	if *runner.siteID != "site-1" {
		t.Errorf("Expected siteID 'site-1', got '%s'", *runner.siteID)
	}
}

// Most tasks are server-scoped, so callers pass through with no site. An
// empty ID must leave the task unscoped rather than storing "".
func TestTaskRunner_ForSiteIgnoresEmptyID(t *testing.T) {
	runner := NewTaskRunner(createTestServer(), createTestTask()).ForSite("")

	if runner.siteID != nil {
		t.Errorf("Expected siteID to stay nil, got '%s'", *runner.siteID)
	}
}

// The scope has to reach the persisted row — that column is what Active
// Actions joins on to target the site.
func TestTaskRunner_ForSiteReachesTheTaskModel(t *testing.T) {
	db := asyncRunnerDatabase(t)
	server := createTestServer()

	scoped, err := NewTaskRunner(server, createTestTask()).
		WithDB(db).
		ForSite("site-1").
		createTaskModel()
	if err != nil {
		t.Fatalf("createTaskModel() = %v", err)
	}
	if scoped.SiteID == nil || *scoped.SiteID != "site-1" {
		t.Errorf("Expected task model SiteID 'site-1', got %v", scoped.SiteID)
	}

	unscoped, err := NewTaskRunner(server, createTestTask()).WithDB(db).createTaskModel()
	if err != nil {
		t.Fatalf("createTaskModel() = %v", err)
	}
	if unscoped.SiteID != nil {
		t.Errorf("Expected server-scoped task to have no SiteID, got %v", *unscoped.SiteID)
	}
}
