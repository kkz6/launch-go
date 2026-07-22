package tasks_test

import (
	"os"
	"testing"

	"github.com/kkz6/launch-go/internal/bootstrap/tasktemplates"
)

func TestMain(m *testing.M) {
	tasktemplates.MustRegisterAll()
	os.Exit(m.Run())
}
