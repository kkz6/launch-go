package tasks

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
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
	t.onSuccessCalled = true
	t.lastTaskID = taskID
	return t.returnError
}

func (t *mockCallbackTask) OnFailure(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string, exitCode int) error {
	t.onFailureCalled = true
	t.lastTaskID = taskID
	t.lastExitCode = exitCode
	return t.returnError
}

func (t *mockCallbackTask) OnExpired(ctx context.Context, cbCtx *taskrunner.CallbackContext, taskID string) error {
	t.onExpiredCalled = true
	t.lastTaskID = taskID
	return t.returnError
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

// =============================================================================
// TaskStatus Tests
// =============================================================================

func TestTaskStatus_Constants(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskStatusPending, "pending"},
		{TaskStatusRunning, "running"},
		{TaskStatusFinished, "finished"},
		{TaskStatusFailed, "failed"},
		{TaskStatusTimeout, "timeout"},
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
