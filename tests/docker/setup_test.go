package docker_test

import (
	"os"
	"testing"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

func TestMain(m *testing.M) {
	templates.MustRegisterAll()
	os.Exit(m.Run())
}
