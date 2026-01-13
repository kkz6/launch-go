package testutil

import (
	"io"
	"testing"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TestJobContext provides a configured context for testing with FakeDispatcher.
// Note: This is a simplified test context that doesn't include database operations.
// For full integration tests with database, use a separate test database setup.
type TestJobContext struct {
	Repo       *MockRepository
	Dispatcher *taskrunner.FakeDispatcher
	Logger     zerolog.Logger
}

// NewTestJobContext creates a new test job context with mock dependencies.
func NewTestJobContext() *TestJobContext {
	repo := NewMockRepository()
	dispatcher := taskrunner.NewFakeDispatcher()
	logger := zerolog.New(io.Discard)

	return &TestJobContext{
		Repo:       repo,
		Dispatcher: dispatcher,
		Logger:     logger,
	}
}

// AssertDispatcher returns an assertion helper for the dispatcher.
func (t *TestJobContext) AssertDispatcher(tt *testing.T) *DispatcherAssertion {
	return &DispatcherAssertion{t: tt, d: t.Dispatcher}
}
