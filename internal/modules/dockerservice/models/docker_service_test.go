package models

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
)

func TestDockerService_IsRunning(t *testing.T) {
	cases := map[types.Status]bool{
		types.StatusRunning:    true,
		types.StatusStopped:    false,
		types.StatusFailed:     false,
		types.StatusInstalling: false,
		types.StatusPending:    false,
	}
	for status, want := range cases {
		m := &DockerService{Status: status}
		if got := m.IsRunning(); got != want {
			t.Errorf("IsRunning(%s) = %v, want %v", status, got, want)
		}
	}
}

func TestDockerService_IsStopped(t *testing.T) {
	m := &DockerService{Status: types.StatusStopped}
	if !m.IsStopped() {
		t.Error("expected IsStopped to be true")
	}
	m.Status = types.StatusRunning
	if m.IsStopped() {
		t.Error("running service must not be stopped")
	}
}

func TestDockerService_IsBusy(t *testing.T) {
	for _, s := range []types.Status{types.StatusInstalling, types.StatusPending} {
		m := &DockerService{Status: s}
		if !m.IsBusy() {
			t.Errorf("status %s must be busy", s)
		}
	}
	for _, s := range []types.Status{types.StatusRunning, types.StatusStopped, types.StatusFailed} {
		m := &DockerService{Status: s}
		if m.IsBusy() {
			t.Errorf("status %s must not be busy", s)
		}
	}
}

func TestDockerService_TableName(t *testing.T) {
	if (DockerService{}).TableName() != "docker_services" {
		t.Errorf("unexpected table name: %s", (DockerService{}).TableName())
	}
}
