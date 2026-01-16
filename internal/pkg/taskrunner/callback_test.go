package taskrunner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// =============================================================================
// Test Fixtures
// =============================================================================

// testState represents a simple state struct for testing
type testState struct {
	SiteID       string `json:"site_id"`
	DeploymentID string `json:"deployment_id"`
}

// testCallbackTask is a mock task that implements CallbackHandler
type testCallbackTask struct {
	state            testState
	onSuccessCalled  bool
	onFailureCalled  bool
	onExpiredCalled  bool
	lastTaskID       string
	lastExitCode     int
	returnError      error
}

func (t *testCallbackTask) OnSuccess(ctx context.Context, cbCtx *CallbackContext, taskID string) error {
	t.onSuccessCalled = true
	t.lastTaskID = taskID
	return t.returnError
}

func (t *testCallbackTask) OnFailure(ctx context.Context, cbCtx *CallbackContext, taskID string, exitCode int) error {
	t.onFailureCalled = true
	t.lastTaskID = taskID
	t.lastExitCode = exitCode
	return t.returnError
}

func (t *testCallbackTask) OnExpired(ctx context.Context, cbCtx *CallbackContext, taskID string) error {
	t.onExpiredCalled = true
	t.lastTaskID = taskID
	return t.returnError
}

// testCallbackPayloadTask implements both CallbackHandler and CallbackPayload
type testCallbackPayloadTask struct {
	*testCallbackTask
	typeName string
}

func (t *testCallbackPayloadTask) TypeName() string {
	return t.typeName
}

func (t *testCallbackPayloadTask) MarshalPayload() ([]byte, error) {
	return json.Marshal(t.state)
}

// stateWithFactory implements CallbackStateFactory
type stateWithFactory struct {
	SiteID       string `json:"site_id"`
	DeploymentID string `json:"deployment_id"`
}

func (s stateWithFactory) NewTask() CallbackHandler {
	return &testCallbackTask{
		state: testState{
			SiteID:       s.SiteID,
			DeploymentID: s.DeploymentID,
		},
	}
}

// =============================================================================
// TaskTypeRegistry Tests
// =============================================================================

func TestNewTaskTypeRegistry(t *testing.T) {
	registry := NewTaskTypeRegistry()
	if registry == nil {
		t.Fatal("Expected registry to be created")
	}
	if registry.factories == nil {
		t.Fatal("Expected factories map to be initialized")
	}
}

func TestTaskTypeRegistry_Register(t *testing.T) {
	registry := NewTaskTypeRegistry()

	factory := func(payload []byte) (CallbackHandler, error) {
		return &testCallbackTask{}, nil
	}

	registry.Register("test:task", factory)

	if !registry.HasType("test:task") {
		t.Error("Expected type to be registered")
	}
}

func TestTaskTypeRegistry_HasType(t *testing.T) {
	registry := NewTaskTypeRegistry()

	// Should not have unregistered type
	if registry.HasType("unknown:type") {
		t.Error("Expected HasType to return false for unregistered type")
	}

	// Register and check
	registry.Register("test:type", func(payload []byte) (CallbackHandler, error) {
		return &testCallbackTask{}, nil
	})

	if !registry.HasType("test:type") {
		t.Error("Expected HasType to return true for registered type")
	}
}

func TestTaskTypeRegistry_Reconstruct(t *testing.T) {
	registry := NewTaskTypeRegistry()

	// Register a factory that creates task from JSON payload
	registry.Register("test:reconstruct", func(payload []byte) (CallbackHandler, error) {
		var state testState
		if err := json.Unmarshal(payload, &state); err != nil {
			return nil, err
		}
		return &testCallbackTask{state: state}, nil
	})

	// Test reconstruction
	payload, _ := json.Marshal(testState{SiteID: "site-123", DeploymentID: "deploy-456"})
	handler, err := registry.Reconstruct("test:reconstruct", payload)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	task, ok := handler.(*testCallbackTask)
	if !ok {
		t.Fatal("Expected *testCallbackTask type")
	}

	if task.state.SiteID != "site-123" {
		t.Errorf("Expected SiteID 'site-123', got '%s'", task.state.SiteID)
	}
	if task.state.DeploymentID != "deploy-456" {
		t.Errorf("Expected DeploymentID 'deploy-456', got '%s'", task.state.DeploymentID)
	}
}

func TestTaskTypeRegistry_Reconstruct_UnknownType(t *testing.T) {
	registry := NewTaskTypeRegistry()

	_, err := registry.Reconstruct("unknown:type", []byte("{}"))

	if err == nil {
		t.Fatal("Expected error for unknown type")
	}

	expectedMsg := "unknown task type: unknown:type"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestTaskTypeRegistry_Reconstruct_InvalidPayload(t *testing.T) {
	registry := NewTaskTypeRegistry()

	registry.Register("test:invalid", func(payload []byte) (CallbackHandler, error) {
		var state testState
		if err := json.Unmarshal(payload, &state); err != nil {
			return nil, err
		}
		return &testCallbackTask{state: state}, nil
	})

	// Pass invalid JSON
	_, err := registry.Reconstruct("test:invalid", []byte("not json"))

	if err == nil {
		t.Fatal("Expected error for invalid payload")
	}
}

// =============================================================================
// Default Registry Tests
// =============================================================================

func TestDefaultRegistry_Register(t *testing.T) {
	// Save and restore default registry
	original := DefaultRegistry
	defer func() { DefaultRegistry = original }()
	DefaultRegistry = NewTaskTypeRegistry()

	Register("global:test", func(payload []byte) (CallbackHandler, error) {
		return &testCallbackTask{}, nil
	})

	if !DefaultRegistry.HasType("global:test") {
		t.Error("Expected type to be registered in default registry")
	}
}

func TestDefaultRegistry_Reconstruct(t *testing.T) {
	// Save and restore default registry
	original := DefaultRegistry
	defer func() { DefaultRegistry = original }()
	DefaultRegistry = NewTaskTypeRegistry()

	Register("global:reconstruct", func(payload []byte) (CallbackHandler, error) {
		return &testCallbackTask{state: testState{SiteID: "global-site"}}, nil
	})

	handler, err := Reconstruct("global:reconstruct", []byte("{}"))

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	task := handler.(*testCallbackTask)
	if task.state.SiteID != "global-site" {
		t.Errorf("Expected SiteID 'global-site', got '%s'", task.state.SiteID)
	}
}

// =============================================================================
// RegisterCallback Tests
// =============================================================================

func TestRegisterCallback(t *testing.T) {
	// Save and restore default registry
	original := DefaultRegistry
	defer func() { DefaultRegistry = original }()
	DefaultRegistry = NewTaskTypeRegistry()

	// Register using generics
	RegisterCallback("test:generic", func(state testState) *testCallbackTask {
		return &testCallbackTask{state: state}
	})

	// Test reconstruction
	payload, _ := json.Marshal(testState{SiteID: "site-abc", DeploymentID: "deploy-xyz"})
	handler, err := Reconstruct("test:generic", payload)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	task := handler.(*testCallbackTask)
	if task.state.SiteID != "site-abc" {
		t.Errorf("Expected SiteID 'site-abc', got '%s'", task.state.SiteID)
	}
	if task.state.DeploymentID != "deploy-xyz" {
		t.Errorf("Expected DeploymentID 'deploy-xyz', got '%s'", task.state.DeploymentID)
	}
}

// =============================================================================
// RegisterCallbackState Tests
// =============================================================================

func TestRegisterCallbackState(t *testing.T) {
	// Save and restore default registry
	original := DefaultRegistry
	defer func() { DefaultRegistry = original }()
	DefaultRegistry = NewTaskTypeRegistry()

	// Register using state factory pattern
	RegisterCallbackState[stateWithFactory]("test:statefactory")

	// Test reconstruction
	payload, _ := json.Marshal(stateWithFactory{SiteID: "factory-site", DeploymentID: "factory-deploy"})
	handler, err := Reconstruct("test:statefactory", payload)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	task := handler.(*testCallbackTask)
	if task.state.SiteID != "factory-site" {
		t.Errorf("Expected SiteID 'factory-site', got '%s'", task.state.SiteID)
	}
}

// =============================================================================
// MarshalInstance Tests
// =============================================================================

func TestMarshalInstance(t *testing.T) {
	task := &testCallbackPayloadTask{
		testCallbackTask: &testCallbackTask{
			state: testState{SiteID: "marshal-site", DeploymentID: "marshal-deploy"},
		},
		typeName: "test:marshal",
	}

	instanceData, err := MarshalInstance(task)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify it's valid JSON
	var instance TaskInstance
	if err := json.Unmarshal([]byte(instanceData), &instance); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if instance.TypeName != "test:marshal" {
		t.Errorf("Expected TypeName 'test:marshal', got '%s'", instance.TypeName)
	}

	// Verify payload contains expected data
	var state testState
	if err := json.Unmarshal(instance.Payload, &state); err != nil {
		t.Fatalf("Expected valid payload, got error: %v", err)
	}

	if state.SiteID != "marshal-site" {
		t.Errorf("Expected SiteID 'marshal-site', got '%s'", state.SiteID)
	}
}

// =============================================================================
// UnmarshalInstance Tests
// =============================================================================

func TestUnmarshalInstance(t *testing.T) {
	data := `{"type":"test:unmarshal","payload":{"site_id":"unmarshal-site","deployment_id":"unmarshal-deploy"}}`

	instance, err := UnmarshalInstance(data)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if instance.TypeName != "test:unmarshal" {
		t.Errorf("Expected TypeName 'test:unmarshal', got '%s'", instance.TypeName)
	}
}

func TestUnmarshalInstance_EmptyString(t *testing.T) {
	instance, err := UnmarshalInstance("")

	if err != nil {
		t.Fatalf("Expected no error for empty string, got: %v", err)
	}

	if instance != nil {
		t.Error("Expected nil instance for empty string")
	}
}

func TestUnmarshalInstance_InvalidJSON(t *testing.T) {
	_, err := UnmarshalInstance("not json")

	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

// =============================================================================
// ReconstructFromInstance Tests
// =============================================================================

func TestReconstructFromInstance(t *testing.T) {
	// Save and restore default registry
	original := DefaultRegistry
	defer func() { DefaultRegistry = original }()
	DefaultRegistry = NewTaskTypeRegistry()

	// Register the type
	RegisterCallback("test:fullrecon", func(state testState) *testCallbackTask {
		return &testCallbackTask{state: state}
	})

	// Create instance data
	data := `{"type":"test:fullrecon","payload":{"site_id":"recon-site","deployment_id":"recon-deploy"}}`

	handler, err := ReconstructFromInstance(data)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	task := handler.(*testCallbackTask)
	if task.state.SiteID != "recon-site" {
		t.Errorf("Expected SiteID 'recon-site', got '%s'", task.state.SiteID)
	}
}

func TestReconstructFromInstance_EmptyString(t *testing.T) {
	handler, err := ReconstructFromInstance("")

	if err != nil {
		t.Fatalf("Expected no error for empty string, got: %v", err)
	}

	if handler != nil {
		t.Error("Expected nil handler for empty string")
	}
}

// =============================================================================
// CallbackHandler Interface Tests
// =============================================================================

func TestCallbackHandler_OnSuccess(t *testing.T) {
	task := &testCallbackTask{}
	ctx := context.Background()
	cbCtx := &CallbackContext{}

	err := task.OnSuccess(ctx, cbCtx, "task-123")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !task.onSuccessCalled {
		t.Error("Expected OnSuccess to be called")
	}

	if task.lastTaskID != "task-123" {
		t.Errorf("Expected taskID 'task-123', got '%s'", task.lastTaskID)
	}
}

func TestCallbackHandler_OnFailure(t *testing.T) {
	task := &testCallbackTask{}
	ctx := context.Background()
	cbCtx := &CallbackContext{}

	err := task.OnFailure(ctx, cbCtx, "task-456", 127)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !task.onFailureCalled {
		t.Error("Expected OnFailure to be called")
	}

	if task.lastTaskID != "task-456" {
		t.Errorf("Expected taskID 'task-456', got '%s'", task.lastTaskID)
	}

	if task.lastExitCode != 127 {
		t.Errorf("Expected exitCode 127, got %d", task.lastExitCode)
	}
}

func TestCallbackHandler_OnExpired(t *testing.T) {
	task := &testCallbackTask{}
	ctx := context.Background()
	cbCtx := &CallbackContext{}

	err := task.OnExpired(ctx, cbCtx, "task-789")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !task.onExpiredCalled {
		t.Error("Expected OnExpired to be called")
	}

	if task.lastTaskID != "task-789" {
		t.Errorf("Expected taskID 'task-789', got '%s'", task.lastTaskID)
	}
}

func TestCallbackHandler_ReturnsError(t *testing.T) {
	expectedError := errors.New("callback failed")
	task := &testCallbackTask{returnError: expectedError}
	ctx := context.Background()
	cbCtx := &CallbackContext{}

	err := task.OnSuccess(ctx, cbCtx, "task-err")

	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

// =============================================================================
// CallbackContext Tests
// =============================================================================

func TestCallbackContext_DispatchJob_NoQueue(t *testing.T) {
	cbCtx := &CallbackContext{
		Queue: nil,
	}

	err := cbCtx.DispatchJob("test:job", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected error when queue is nil")
	}

	if err.Error() != "queue not available" {
		t.Errorf("Expected 'queue not available' error, got: %s", err.Error())
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestTaskTypeRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewTaskTypeRegistry()

	// Concurrent registration
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			typeName := "test:concurrent:" + string(rune('a'+n))
			registry.Register(typeName, func(payload []byte) (CallbackHandler, error) {
				return &testCallbackTask{}, nil
			})
			done <- true
		}(i)
	}

	// Wait for all registrations
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all types were registered
	for i := 0; i < 10; i++ {
		typeName := "test:concurrent:" + string(rune('a'+i))
		if !registry.HasType(typeName) {
			t.Errorf("Expected type '%s' to be registered", typeName)
		}
	}
}

// =============================================================================
// End-to-End Tests
// =============================================================================

func TestEndToEnd_RegisterMarshalReconstruct(t *testing.T) {
	// Save and restore default registry
	original := DefaultRegistry
	defer func() { DefaultRegistry = original }()
	DefaultRegistry = NewTaskTypeRegistry()

	// 1. Register the task type
	RegisterCallbackState[stateWithFactory]("e2e:deploy")

	// 2. Create a task with payload
	task := &testCallbackPayloadTask{
		testCallbackTask: &testCallbackTask{
			state: testState{SiteID: "e2e-site", DeploymentID: "e2e-deploy"},
		},
		typeName: "e2e:deploy",
	}

	// 3. Marshal for storage
	instanceData, err := MarshalInstance(task)
	if err != nil {
		t.Fatalf("MarshalInstance failed: %v", err)
	}

	// 4. Later: Reconstruct from storage
	handler, err := ReconstructFromInstance(instanceData)
	if err != nil {
		t.Fatalf("ReconstructFromInstance failed: %v", err)
	}

	// 5. Invoke callback
	ctx := context.Background()
	cbCtx := &CallbackContext{}
	err = handler.OnSuccess(ctx, cbCtx, "task-e2e")

	if err != nil {
		t.Fatalf("OnSuccess failed: %v", err)
	}

	// Verify callback was invoked
	reconstructed := handler.(*testCallbackTask)
	if !reconstructed.onSuccessCalled {
		t.Error("Expected OnSuccess to be called on reconstructed task")
	}
}
