package tasks

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemUpdatesHostProgram(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required to exercise the host update program")
	}
	output, err := exec.Command(python, "-B", "system_updates_test.py", "-v").CombinedOutput()
	require.NoError(t, err, "%s", output)
}
