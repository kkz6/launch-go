package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestTaskGetLogPathMatchesServerScriptPath(t *testing.T) {
	customWorkingDirectory := "launch-work"
	awsServer := &Server{Provider: servertypes.ProviderAWS}

	tests := []struct {
		name     string
		server   *Server
		user     string
		expected string
	}{
		{
			name:     "root user",
			server:   &Server{},
			user:     "root",
			expected: "/root/.launch/task-task-01.log",
		},
		{
			name:     "AWS root connection uses ubuntu home",
			server:   awsServer,
			user:     awsServer.RootUsername(),
			expected: "/home/ubuntu/.launch/task-task-01.log",
		},
		{
			name:     "custom task user",
			server:   &Server{},
			user:     "deploy",
			expected: "/home/deploy/.launch/task-task-01.log",
		},
		{
			name: "custom working directory",
			server: &Server{
				WorkingDirectory: &customWorkingDirectory,
			},
			user:     "ubuntu",
			expected: "/home/ubuntu/launch-work/task-task-01.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				BaseModel: basemodels.BaseModel{ID: "task-01"},
				User:      tt.user,
				Server:    tt.server,
			}

			require.Equal(t, tt.expected, task.GetLogPath())
			require.Equal(
				t,
				tt.server.GetScriptPath(tt.user)+"/task-task-01.log",
				task.GetLogPath(),
			)
		})
	}
}

func TestTaskGetLogPathUsesDefaultServerPathWithoutPreload(t *testing.T) {
	task := &Task{
		BaseModel: basemodels.BaseModel{ID: "task-02"},
		User:      "ubuntu",
	}

	require.Equal(t, "/home/ubuntu/.launch/task-task-02.log", task.GetLogPath())
}
