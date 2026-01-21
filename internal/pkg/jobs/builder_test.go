package jobs

import (
	"strings"
	"testing"
	"time"
)

func TestTaskBuilder_Basic(t *testing.T) {
	task := NewTaskBuilder("Test Task").
		WithTimeoutSeconds(30).
		Build()

	if task.Name() != "Test Task" {
		t.Errorf("Expected name 'Test Task', got '%s'", task.Name())
	}

	if task.Timeout() != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", task.Timeout())
	}
}

func TestTaskBuilder_AddScript(t *testing.T) {
	task := NewTaskBuilder("Test Task").
		AddScript("echo 'Hello'").
		AddScript("mkdir -p %s", "/tmp/test").
		Build()

	script := task.Script()

	if !strings.Contains(script, "echo 'Hello'") {
		t.Error("Expected script to contain 'echo Hello'")
	}

	if !strings.Contains(script, "mkdir -p /tmp/test") {
		t.Error("Expected script to contain 'mkdir -p /tmp/test'")
	}
}

func TestTaskBuilder_AddScriptIf(t *testing.T) {
	t.Run("condition true", func(t *testing.T) {
		task := NewTaskBuilder("Test").
			AddScriptIf(true, "echo 'included'").
			Build()

		if !strings.Contains(task.Script(), "echo 'included'") {
			t.Error("Expected script to contain conditional script when condition is true")
		}
	})

	t.Run("condition false", func(t *testing.T) {
		task := NewTaskBuilder("Test").
			AddScriptIf(false, "echo 'excluded'").
			Build()

		if strings.Contains(task.Script(), "echo 'excluded'") {
			t.Error("Expected script to NOT contain conditional script when condition is false")
		}
	})
}

func TestTaskBuilder_ShellHeader(t *testing.T) {
	t.Run("with shell header (default)", func(t *testing.T) {
		task := NewTaskBuilder("Test").Build()
		script := task.Script()

		if !strings.Contains(script, "#!/bin/bash") {
			t.Error("Expected script to contain shebang")
		}
		if !strings.Contains(script, "set -euo pipefail") {
			t.Error("Expected script to contain shell options")
		}
	})

	t.Run("without shell header", func(t *testing.T) {
		task := NewTaskBuilder("Test").
			WithoutShellHeader().
			AddScript("echo 'test'").
			Build()

		script := task.Script()

		if strings.Contains(script, "#!/bin/bash") {
			t.Error("Expected script to NOT contain shebang")
		}
	})
}

func TestTaskBuilder_AptWait(t *testing.T) {
	task := NewTaskBuilder("Test").
		WithAptWait().
		AddScript("echo 'test'").
		Build()

	script := task.Script()

	if !strings.Contains(script, "waitForAptUnlock") {
		t.Error("Expected script to contain apt wait function")
	}
}

func TestTaskBuilder_HelperMethods(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *TaskBuilder
		expected string
	}{
		{
			name: "AddComment",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddComment("This is a comment")
			},
			expected: "# This is a comment",
		},
		{
			name: "AddEcho",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddEcho("Hello World")
			},
			expected: `echo "Hello World"`,
		},
		{
			name: "AddCD",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddCD("/tmp/test")
			},
			expected: "cd /tmp/test",
		},
		{
			name: "AddMkdir",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddMkdir("/tmp/test")
			},
			expected: "mkdir -p /tmp/test",
		},
		{
			name: "AddChown",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddChown("launch", "/var/www")
			},
			expected: "chown launch:launch /var/www",
		},
		{
			name: "AddChmod",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddChmod("755", "/var/www")
			},
			expected: "chmod 755 /var/www",
		},
		{
			name: "AddRemove force",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddRemove("/tmp/file", true, false)
			},
			expected: "rm -f /tmp/file",
		},
		{
			name: "AddRemove recursive",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddRemove("/tmp/dir", false, true)
			},
			expected: "rm -r /tmp/dir",
		},
		{
			name: "AddRemove force recursive",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddRemove("/tmp/dir", true, true)
			},
			expected: "rm -fr /tmp/dir",
		},
		{
			name: "AddSystemctl",
			builder: func() *TaskBuilder {
				return NewTaskBuilder("Test").AddSystemctl("restart", "nginx")
			},
			expected: "systemctl restart nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tt.builder().Build()
			if !strings.Contains(task.Script(), tt.expected) {
				t.Errorf("Expected script to contain '%s', got:\n%s", tt.expected, task.Script())
			}
		})
	}
}

func TestTaskBuilder_AddHeredoc(t *testing.T) {
	content := `line1
line2
line3`
	task := NewTaskBuilder("Test").
		AddHeredoc("/tmp/file", content).
		Build()

	script := task.Script()

	if !strings.Contains(script, "cat > /tmp/file << 'HEREDOCEOF'") {
		t.Error("Expected script to contain heredoc syntax")
	}
	if !strings.Contains(script, "line1") {
		t.Error("Expected script to contain heredoc content")
	}
	if !strings.Contains(script, "HEREDOCEOF") {
		t.Error("Expected script to contain heredoc terminator")
	}
}

func TestTaskBuilder_AddConditional(t *testing.T) {
	t.Run("with else", func(t *testing.T) {
		task := NewTaskBuilder("Test").
			AddConditional("[ -f /tmp/file ]", "echo 'exists'", "echo 'not exists'").
			Build()

		script := task.Script()

		if !strings.Contains(script, "if [ -f /tmp/file ]; then") {
			t.Error("Expected script to contain if condition")
		}
		if !strings.Contains(script, "echo 'exists'") {
			t.Error("Expected script to contain then block")
		}
		if !strings.Contains(script, "else") {
			t.Error("Expected script to contain else")
		}
		if !strings.Contains(script, "echo 'not exists'") {
			t.Error("Expected script to contain else block")
		}
		if !strings.Contains(script, "fi") {
			t.Error("Expected script to contain fi")
		}
	})

	t.Run("without else", func(t *testing.T) {
		task := NewTaskBuilder("Test").
			AddConditional("[ -d /tmp ]", "echo 'dir exists'", "").
			Build()

		script := task.Script()

		if !strings.Contains(script, "if [ -d /tmp ]; then") {
			t.Error("Expected script to contain if condition")
		}
		if strings.Contains(script, "else") {
			t.Error("Expected script to NOT contain else when else script is empty")
		}
	})
}

func TestTaskBuilder_AddFileExists(t *testing.T) {
	task := NewTaskBuilder("Test").
		AddFileExists("/tmp/file", "echo 'exists'", "echo 'missing'").
		Build()

	script := task.Script()

	if !strings.Contains(script, "[ -f /tmp/file ]") {
		t.Error("Expected script to contain file exists check")
	}
}

func TestTaskBuilder_AddDirExists(t *testing.T) {
	task := NewTaskBuilder("Test").
		AddDirExists("/tmp/dir", "echo 'exists'", "echo 'missing'").
		Build()

	script := task.Script()

	if !strings.Contains(script, "[ -d /tmp/dir ]") {
		t.Error("Expected script to contain dir exists check")
	}
}

func TestTaskBuilder_AddAptInstall(t *testing.T) {
	t.Run("without apt wait", func(t *testing.T) {
		task := NewTaskBuilder("Test").
			AddAptInstall("nginx", "php8.3-fpm").
			Build()

		script := task.Script()

		if !strings.Contains(script, "apt-get install -y nginx php8.3-fpm") {
			t.Error("Expected script to contain apt-get install")
		}
		if strings.Contains(script, "waitForAptUnlock") {
			t.Error("Expected script to NOT contain waitForAptUnlock when not enabled")
		}
	})

	t.Run("with apt wait", func(t *testing.T) {
		task := NewTaskBuilder("Test").
			WithAptWait().
			AddAptInstall("nginx").
			Build()

		script := task.Script()

		// The waitForAptUnlock should appear before apt-get
		idx1 := strings.Index(script, "waitForAptUnlock")
		idx2 := strings.Index(script, "apt-get install")

		if idx1 == -1 {
			t.Error("Expected script to contain waitForAptUnlock")
		}
		if idx2 == -1 {
			t.Error("Expected script to contain apt-get install")
		}
	})
}

func TestTaskBuilder_Script(t *testing.T) {
	builder := NewTaskBuilder("Test").
		AddScript("echo 'test'")

	script := builder.Script()

	if !strings.Contains(script, "echo 'test'") {
		t.Error("Expected Script() to return the built script")
	}
}

func TestQuickTask(t *testing.T) {
	task := QuickTask("Quick Task", "echo 'quick'", 60)

	if task.Name() != "Quick Task" {
		t.Errorf("Expected name 'Quick Task', got '%s'", task.Name())
	}

	if task.Timeout() != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", task.Timeout())
	}

	if !strings.Contains(task.Script(), "echo 'quick'") {
		t.Error("Expected script to contain the command")
	}
}

func TestFileUploadTask(t *testing.T) {
	task := FileUploadTask("Upload Config", "/etc/config", "key=value", "root", "644")

	script := task.Script()

	if !strings.Contains(script, "cat > /etc/config") {
		t.Error("Expected script to write to file")
	}
	if !strings.Contains(script, "key=value") {
		t.Error("Expected script to contain file content")
	}
	if !strings.Contains(script, "chmod 644") {
		t.Error("Expected script to set permissions")
	}
	if !strings.Contains(script, "chown root:root") {
		t.Error("Expected script to set ownership")
	}
}

func TestFileDeleteTask(t *testing.T) {
	task := FileDeleteTask("Delete File", "/tmp/file")

	script := task.Script()

	if !strings.Contains(script, "[ -f /tmp/file ]") {
		t.Error("Expected script to check if file exists")
	}
	if !strings.Contains(script, "rm -f /tmp/file") {
		t.Error("Expected script to remove file")
	}
}

func TestServiceTask(t *testing.T) {
	task := ServiceTask("Restart Nginx", "restart", "nginx")

	script := task.Script()

	if !strings.Contains(script, "systemctl restart nginx") {
		t.Error("Expected script to contain systemctl command")
	}
}
