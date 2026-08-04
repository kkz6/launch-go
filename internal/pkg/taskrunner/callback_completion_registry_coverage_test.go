package taskrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type coreTestNotification string

func (n coreTestNotification) RawText() string { return string(n) }

type coreTestNotifier struct {
	teamID       string
	channelID    string
	notification Notification
	teamErr      error
	channelErr   error
}

func (n *coreTestNotifier) SendToTeam(
	_ context.Context,
	teamID string,
	notification Notification,
) error {
	n.teamID = teamID
	n.notification = notification
	return n.teamErr
}

func (n *coreTestNotifier) SendToChannel(
	_ context.Context,
	channelID string,
	notification Notification,
) error {
	n.channelID = channelID
	n.notification = notification
	return n.channelErr
}

type corePayloadTask struct {
	*testCallbackTask
	typeName string
	payload  []byte
	err      error
}

func (t *corePayloadTask) TypeName() string { return t.typeName }

func (t *corePayloadTask) MarshalPayload() ([]byte, error) {
	return t.payload, t.err
}

type coreCompletionTask struct {
	*BaseTask
	jobType string
	payload any
}

func (t *coreCompletionTask) CompletionJobType() string { return t.jobType }

func (t *coreCompletionTask) CompletionPayload() interface{} { return t.payload }

func TestCallbackContextNotifications(t *testing.T) {
	ctx := context.Background()
	notification := coreTestNotification("deployment complete")

	t.Run("silently skips without notifier or logger", func(t *testing.T) {
		cbCtx := &CallbackContext{}
		if err := cbCtx.NotifyTeam(ctx, "team-1", notification); err != nil {
			t.Fatalf("NotifyTeam() error = %v", err)
		}
		if err := cbCtx.NotifyChannel(ctx, "channel-1", notification); err != nil {
			t.Fatalf("NotifyChannel() error = %v", err)
		}
	})

	t.Run("silently skips and logs without notifier", func(t *testing.T) {
		log := zerolog.Nop()
		cbCtx := &CallbackContext{Logger: &log}
		if err := cbCtx.NotifyTeam(ctx, "team-2", notification); err != nil {
			t.Fatalf("NotifyTeam() error = %v", err)
		}
		if err := cbCtx.NotifyChannel(ctx, "channel-2", notification); err != nil {
			t.Fatalf("NotifyChannel() error = %v", err)
		}
	})

	t.Run("delegates and preserves notifier errors", func(t *testing.T) {
		teamErr := errors.New("team send failed")
		channelErr := errors.New("channel send failed")
		notifier := &coreTestNotifier{teamErr: teamErr, channelErr: channelErr}
		cbCtx := &CallbackContext{Notifier: notifier}

		if err := cbCtx.NotifyTeam(ctx, "team-3", notification); !errors.Is(err, teamErr) {
			t.Fatalf("NotifyTeam() error = %v, want %v", err, teamErr)
		}
		if notifier.teamID != "team-3" || notifier.notification != notification {
			t.Fatalf("team notification = (%q, %v)", notifier.teamID, notifier.notification)
		}

		if err := cbCtx.NotifyChannel(ctx, "channel-3", notification); !errors.Is(err, channelErr) {
			t.Fatalf("NotifyChannel() error = %v, want %v", err, channelErr)
		}
		if notifier.channelID != "channel-3" || notifier.notification != notification {
			t.Fatalf("channel notification = (%q, %v)", notifier.channelID, notifier.notification)
		}
	})
}

func TestCallbackContextGetTaskOutputTail(t *testing.T) {
	if got := (&CallbackContext{}).GetTaskOutputTail("missing", 2); got != "" {
		t.Fatalf("nil DB tail = %q, want empty", got)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec("CREATE TABLE tasks (id TEXT PRIMARY KEY, output TEXT)").Error; err != nil {
		t.Fatalf("create tasks table: %v", err)
	}
	for _, row := range []struct {
		id     string
		output string
	}{
		{id: "empty", output: ""},
		{id: "short", output: "one\ntwo"},
		{id: "long", output: "one\ntwo\nthree\nfour"},
	} {
		if err := db.Exec("INSERT INTO tasks (id, output) VALUES (?, ?)", row.id, row.output).Error; err != nil {
			t.Fatalf("insert %s: %v", row.id, err)
		}
	}

	cbCtx := &CallbackContext{DB: db}
	for _, tc := range []struct {
		name   string
		taskID string
		lines  int
		want   string
	}{
		{name: "missing task", taskID: "missing", lines: 2, want: ""},
		{name: "empty output", taskID: "empty", lines: 2, want: ""},
		{name: "output shorter than limit", taskID: "short", lines: 3, want: "one\ntwo"},
		{name: "tail is selected", taskID: "long", lines: 2, want: "three\nfour"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cbCtx.GetTaskOutputTail(tc.taskID, tc.lines); got != tc.want {
				t.Fatalf("GetTaskOutputTail() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCallbackSerializationFailurePaths(t *testing.T) {
	t.Run("payload marshaling failure", func(t *testing.T) {
		task := &corePayloadTask{
			testCallbackTask: &testCallbackTask{},
			typeName:         "core:marshal-error",
			err:              errors.New("payload unavailable"),
		}
		if _, err := MarshalInstance(task); err == nil || !strings.Contains(err.Error(), "failed to marshal payload") {
			t.Fatalf("MarshalInstance() error = %v", err)
		}
	})

	t.Run("instance marshaling failure", func(t *testing.T) {
		task := &corePayloadTask{
			testCallbackTask: &testCallbackTask{},
			typeName:         "core:invalid-json",
			payload:          []byte("{"),
		}
		if _, err := MarshalInstance(task); err == nil || !strings.Contains(err.Error(), "failed to marshal instance") {
			t.Fatalf("MarshalInstance() error = %v", err)
		}
	})

	if _, err := ReconstructFromInstance("{"); err == nil {
		t.Fatal("ReconstructFromInstance() accepted invalid instance JSON")
	}
}

func TestCallbackGenericRegistrationFailurePaths(t *testing.T) {
	original := DefaultRegistry
	DefaultRegistry = NewTaskTypeRegistry()
	t.Cleanup(func() { DefaultRegistry = original })

	RegisterCallback("core:generic", func(state testState) *testCallbackTask {
		return &testCallbackTask{state: state}
	})
	if _, err := Reconstruct("core:generic", []byte("{")); err == nil || !strings.Contains(err.Error(), "failed to unmarshal state") {
		t.Fatalf("generic reconstruction error = %v", err)
	}

	RegisterCallbackState[stateWithFactory]("core:state")
	if _, err := Reconstruct("core:state", []byte("{")); err == nil || !strings.Contains(err.Error(), "failed to unmarshal state") {
		t.Fatalf("state reconstruction error = %v", err)
	}
}

func TestCompletionConfiguration(t *testing.T) {
	t.Run("job reference", func(t *testing.T) {
		if ref, err := NewJobRef("", map[string]string{"id": "ignored"}); err != nil || ref != nil {
			t.Fatalf("NewJobRef(empty) = (%v, %v), want (nil, nil)", ref, err)
		}

		ref, err := NewJobRef("site:finished", map[string]string{"site_id": "site-1"})
		if err != nil {
			t.Fatalf("NewJobRef() error = %v", err)
		}
		if ref.Type != "site:finished" || string(ref.Payload) != `{"site_id":"site-1"}` {
			t.Fatalf("NewJobRef() = %#v", ref)
		}

		if _, err := NewJobRef("site:bad", make(chan struct{})); err == nil {
			t.Fatal("NewJobRef() accepted an unsupported payload")
		}
	})

	t.Run("marshal and unmarshal", func(t *testing.T) {
		if got, err := MarshalCompletionConfig(nil); err != nil || got != "" {
			t.Fatalf("MarshalCompletionConfig(nil) = (%q, %v)", got, err)
		}

		config := &CompletionConfig{OnFinished: &JobRef{Type: "site:done", Payload: json.RawMessage(`{"id":"1"}`)}}
		data, err := MarshalCompletionConfig(config)
		if err != nil {
			t.Fatalf("MarshalCompletionConfig() error = %v", err)
		}
		decoded, err := UnmarshalCompletionConfig(data)
		if err != nil {
			t.Fatalf("UnmarshalCompletionConfig() error = %v", err)
		}
		if decoded.OnFinished == nil || decoded.OnFinished.Type != "site:done" {
			t.Fatalf("decoded config = %#v", decoded)
		}

		badConfig := &CompletionConfig{OnFinished: &JobRef{Type: "site:bad", Payload: json.RawMessage("{")}}
		if _, err := MarshalCompletionConfig(badConfig); err == nil {
			t.Fatal("MarshalCompletionConfig() accepted invalid raw JSON")
		}
		if config, err := UnmarshalCompletionConfig(""); err != nil || config != nil {
			t.Fatalf("UnmarshalCompletionConfig(empty) = (%v, %v)", config, err)
		}
		if _, err := UnmarshalCompletionConfig("{"); err == nil {
			t.Fatal("UnmarshalCompletionConfig() accepted invalid JSON")
		}
	})

	t.Run("extraction", func(t *testing.T) {
		plainTask := NewBaseTask()
		if config, err := ExtractCompletionConfig(plainTask); err != nil || config != nil {
			t.Fatalf("ExtractCompletionConfig(plain) = (%v, %v)", config, err)
		}

		empty := &coreCompletionTask{BaseTask: NewBaseTask(), jobType: ""}
		if config, err := ExtractCompletionConfig(empty); err != nil || config != nil {
			t.Fatalf("ExtractCompletionConfig(empty) = (%v, %v)", config, err)
		}

		bad := &coreCompletionTask{BaseTask: NewBaseTask(), jobType: "site:bad", payload: make(chan struct{})}
		if _, err := ExtractCompletionConfig(bad); err == nil {
			t.Fatal("ExtractCompletionConfig() accepted invalid payload")
		}

		valid := &coreCompletionTask{
			BaseTask: NewBaseTask(),
			jobType:  "site:done",
			payload:  map[string]string{"site_id": "site-1"},
		}
		config, err := ExtractCompletionConfig(valid)
		if err != nil {
			t.Fatalf("ExtractCompletionConfig() error = %v", err)
		}
		if config.OnFinished == nil || config.OnFinished.Type != "site:done" || config.OnFailed != nil || config.OnTimeout != nil {
			t.Fatalf("completion config = %#v", config)
		}
	})
}

func TestRegistryLifecycle(t *testing.T) {
	original := DefaultRegistry
	DefaultRegistry = NewTaskTypeRegistry()
	t.Cleanup(func() { DefaultRegistry = original })

	factory := func([]byte) (CallbackHandler, error) { return &testCallbackTask{}, nil }
	MustRegister("core:zeta", factory)
	MustRegister("core:alpha", factory)

	if !IsRegistered("core:zeta") || IsRegistered("core:missing") {
		t.Fatalf("unexpected registry membership: %v", ListRegisteredTypes())
	}
	if got := RegisteredCount(); got != 2 {
		t.Fatalf("RegisteredCount() = %d, want 2", got)
	}
	if got := ListRegisteredTypes(); fmt.Sprint(got) != "[core:alpha core:zeta]" {
		t.Fatalf("ListRegisteredTypes() = %v", got)
	}
	info := GetRegistryInfo()
	if info.TotalCount != 2 || fmt.Sprint(info.Types) != "[core:alpha core:zeta]" {
		t.Fatalf("GetRegistryInfo() = %#v", info)
	}
	if err := ValidateRegistry([]string{"core:alpha", "core:zeta"}); err != nil {
		t.Fatalf("ValidateRegistry() error = %v", err)
	}
	if err := ValidateRegistry([]string{"core:alpha", "core:missing"}); err == nil || !strings.Contains(err.Error(), "core:missing") {
		t.Fatalf("ValidateRegistry() error = %v", err)
	}

	func() {
		defer func() {
			if recovered := recover(); recovered == nil || !strings.Contains(fmt.Sprint(recovered), "already registered") {
				t.Fatalf("duplicate MustRegister() panic = %v", recovered)
			}
		}()
		MustRegister("core:alpha", factory)
	}()

	ClearRegistry()
	if got := RegisteredCount(); got != 0 {
		t.Fatalf("RegisteredCount() after ClearRegistry = %d", got)
	}
}

func TestMustRegisterCallbackState(t *testing.T) {
	original := DefaultRegistry
	DefaultRegistry = NewTaskTypeRegistry()
	t.Cleanup(func() { DefaultRegistry = original })

	MustRegisterCallbackState[stateWithFactory]("core:must-state")
	handler, err := Reconstruct("core:must-state", []byte(`{"site_id":"site-1","deployment_id":"deploy-1"}`))
	if err != nil {
		t.Fatalf("Reconstruct() error = %v", err)
	}
	if task, ok := handler.(*testCallbackTask); !ok || task.state.SiteID != "site-1" {
		t.Fatalf("reconstructed handler = %#v", handler)
	}

	if _, err := Reconstruct("core:must-state", []byte("{")); err == nil || !strings.Contains(err.Error(), "failed to unmarshal state") {
		t.Fatalf("invalid state reconstruction error = %v", err)
	}
}
