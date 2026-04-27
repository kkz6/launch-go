package models

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/managedservice/types"
)

func TestManagedService_IsRunning(t *testing.T) {
	cases := map[types.Status]bool{
		types.StatusRunning:    true,
		types.StatusStopped:    false,
		types.StatusFailed:     false,
		types.StatusInstalling: false,
		types.StatusPending:    false,
	}
	for status, want := range cases {
		m := &ManagedService{Status: status}
		if got := m.IsRunning(); got != want {
			t.Errorf("IsRunning(%s) = %v, want %v", status, got, want)
		}
	}
}

func TestManagedService_IsStopped(t *testing.T) {
	m := &ManagedService{Status: types.StatusStopped}
	if !m.IsStopped() {
		t.Error("expected IsStopped to be true")
	}
	m.Status = types.StatusRunning
	if m.IsStopped() {
		t.Error("running service must not be stopped")
	}
}

func TestManagedService_IsBusy(t *testing.T) {
	for _, s := range []types.Status{types.StatusInstalling, types.StatusPending} {
		m := &ManagedService{Status: s}
		if !m.IsBusy() {
			t.Errorf("status %s must be busy", s)
		}
	}
	for _, s := range []types.Status{types.StatusRunning, types.StatusStopped, types.StatusFailed} {
		m := &ManagedService{Status: s}
		if m.IsBusy() {
			t.Errorf("status %s must not be busy", s)
		}
	}
}

func TestManagedService_TableName(t *testing.T) {
	if (ManagedService{}).TableName() != "managed_services" {
		t.Errorf("unexpected table name: %s", (ManagedService{}).TableName())
	}
}
