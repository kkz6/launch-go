package types

import "testing"

// ----- Source -----

func TestSource_IsValid(t *testing.T) {
	if !SourceImage.IsValid() {
		t.Error("image source must be valid")
	}
	if !SourceCompose.IsValid() {
		t.Error("compose source must be valid")
	}
	if !SourceGit.IsValid() {
		t.Error("git source must be valid")
	}
	if Source("svn").IsValid() {
		t.Error("unknown source must not be valid")
	}
}

// ----- Status -----

func TestStatus_IsTerminal(t *testing.T) {
	for _, s := range []Status{StatusRunning, StatusStopped, StatusFailed} {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range []Status{StatusPending, StatusDeploying} {
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestStatus_IsBusy(t *testing.T) {
	if !StatusDeploying.IsBusy() {
		t.Error("deploying must be busy")
	}
	if !StatusPending.IsBusy() {
		t.Error("pending must be busy")
	}
	if StatusRunning.IsBusy() {
		t.Error("running must not be busy")
	}
}

// ----- RestartPolicy -----

func TestRestartPolicy_IsValid(t *testing.T) {
	for _, p := range AllRestartPolicies() {
		if !p.IsValid() {
			t.Errorf("%s should be valid", p)
		}
	}
	if RestartPolicy("rebooting").IsValid() {
		t.Error("unknown policy should not be valid")
	}
}

// ----- DefaultContainerName -----

func TestDefaultContainerName(t *testing.T) {
	cases := map[string]string{
		"acme-app":  "launch-app-acme-app",
		"my_thing":  "launch-app-my_thing",
		"with-dash": "launch-app-with-dash",
	}
	for in, want := range cases {
		if got := DefaultContainerName(in); got != want {
			t.Errorf("DefaultContainerName(%q) = %q, want %q", in, got, want)
		}
	}
}
