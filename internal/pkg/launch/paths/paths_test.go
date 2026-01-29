package paths

import (
	"testing"
	"time"
)

func TestUser_Home(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "root user",
			username: "root",
			want:     "/root",
		},
		{
			name:     "regular user",
			username: "deploy",
			want:     "/home/deploy",
		},
		{
			name:     "another user",
			username: "ubuntu",
			want:     "/home/ubuntu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := ForUser(tt.username)
			if got := u.Home(); got != tt.want {
				t.Errorf("User.Home() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_SSH(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "root user",
			username: "root",
			want:     "/root/.ssh",
		},
		{
			name:     "regular user",
			username: "deploy",
			want:     "/home/deploy/.ssh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := ForUser(tt.username)
			if got := u.SSH(); got != tt.want {
				t.Errorf("User.SSH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_AuthorizedKeys(t *testing.T) {
	u := ForUser("deploy")
	want := "/home/deploy/.ssh/authorized_keys"
	if got := u.AuthorizedKeys(); got != want {
		t.Errorf("User.AuthorizedKeys() = %v, want %v", got, want)
	}
}

func TestUser_KnownHosts(t *testing.T) {
	u := ForUser("deploy")
	want := "/home/deploy/.ssh/known_hosts"
	if got := u.KnownHosts(); got != want {
		t.Errorf("User.KnownHosts() = %v, want %v", got, want)
	}
}

func TestUser_PrivateKey(t *testing.T) {
	u := ForUser("deploy")
	want := "/home/deploy/.ssh/id_rsa"
	if got := u.PrivateKey("id_rsa"); got != want {
		t.Errorf("User.PrivateKey() = %v, want %v", got, want)
	}
}

func TestUser_PublicKey(t *testing.T) {
	u := ForUser("deploy")
	want := "/home/deploy/.ssh/id_rsa.pub"
	if got := u.PublicKey("id_rsa"); got != want {
		t.Errorf("User.PublicKey() = %v, want %v", got, want)
	}
}

func TestUser_Config(t *testing.T) {
	u := ForUser("deploy")
	want := "/home/deploy/.ssh/config"
	if got := u.Config(); got != want {
		t.Errorf("User.Config() = %v, want %v", got, want)
	}
}

func TestUser_Join(t *testing.T) {
	tests := []struct {
		name     string
		username string
		parts    []string
		want     string
	}{
		{
			name:     "single part",
			username: "deploy",
			parts:    []string{".config"},
			want:     "/home/deploy/.config",
		},
		{
			name:     "multiple parts",
			username: "deploy",
			parts:    []string{".config", "composer"},
			want:     "/home/deploy/.config/composer",
		},
		{
			name:     "root user",
			username: "root",
			parts:    []string{".bashrc"},
			want:     "/root/.bashrc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := ForUser(tt.username)
			if got := u.Join(tt.parts...); got != tt.want {
				t.Errorf("User.Join() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_TaskDir(t *testing.T) {
	u := ForUser("deploy")
	want := "/home/deploy/.launch"
	if got := u.TaskDir(); got != want {
		t.Errorf("User.TaskDir() = %v, want %v", got, want)
	}
}

func TestSite_Root(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com"
	if got := s.Root(); got != want {
		t.Errorf("Site.Root() = %v, want %v", got, want)
	}
}

func TestSite_Current(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/current"
	if got := s.Current(); got != want {
		t.Errorf("Site.Current() = %v, want %v", got, want)
	}
}

func TestSite_Repository(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/repository"
	if got := s.Repository(); got != want {
		t.Errorf("Site.Repository() = %v, want %v", got, want)
	}
}

func TestSite_Releases(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/releases"
	if got := s.Releases(); got != want {
		t.Errorf("Site.Releases() = %v, want %v", got, want)
	}
}

func TestSite_Release(t *testing.T) {
	s := ForSite("deploy", "example.com")
	timestamp := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)
	want := "/home/deploy/example.com/releases/20240115123045"
	if got := s.Release(timestamp); got != want {
		t.Errorf("Site.Release() = %v, want %v", got, want)
	}
}

func TestSite_ReleaseNamed(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/releases/20240115123045"
	if got := s.ReleaseNamed("20240115123045"); got != want {
		t.Errorf("Site.ReleaseNamed() = %v, want %v", got, want)
	}
}

func TestSite_Shared(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/shared"
	if got := s.Shared(); got != want {
		t.Errorf("Site.Shared() = %v, want %v", got, want)
	}
}

func TestSite_Storage(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/shared/storage"
	if got := s.Storage(); got != want {
		t.Errorf("Site.Storage() = %v, want %v", got, want)
	}
}

func TestSite_Env(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/shared/.env"
	if got := s.Env(); got != want {
		t.Errorf("Site.Env() = %v, want %v", got, want)
	}
}

func TestSite_Logs(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/logs"
	if got := s.Logs(); got != want {
		t.Errorf("Site.Logs() = %v, want %v", got, want)
	}
}

func TestSite_Join(t *testing.T) {
	s := ForSite("deploy", "example.com")
	want := "/home/deploy/example.com/config/app.php"
	if got := s.Join("config", "app.php"); got != want {
		t.Errorf("Site.Join() = %v, want %v", got, want)
	}
}

func TestSite_Application(t *testing.T) {
	s := ForSite("deploy", "example.com")

	t.Run("zero downtime", func(t *testing.T) {
		want := "/home/deploy/example.com/current"
		if got := s.Application(true); got != want {
			t.Errorf("Site.Application(true) = %v, want %v", got, want)
		}
	})

	t.Run("standard deployment", func(t *testing.T) {
		want := "/home/deploy/example.com/repository"
		if got := s.Application(false); got != want {
			t.Errorf("Site.Application(false) = %v, want %v", got, want)
		}
	})
}

func TestSite_WebDirectory(t *testing.T) {
	s := ForSite("deploy", "example.com")

	t.Run("zero downtime with public folder", func(t *testing.T) {
		want := "/home/deploy/example.com/current/public"
		if got := s.WebDirectory(true, "public"); got != want {
			t.Errorf("Site.WebDirectory(true, public) = %v, want %v", got, want)
		}
	})

	t.Run("standard deployment with web folder", func(t *testing.T) {
		want := "/home/deploy/example.com/repository/web"
		if got := s.WebDirectory(false, "web"); got != want {
			t.Errorf("Site.WebDirectory(false, web) = %v, want %v", got, want)
		}
	})
}

func TestGetTaskPaths(t *testing.T) {
	paths := GetTaskPaths("/home/deploy/.launch", "abc123")

	if paths.Script != "/home/deploy/.launch/task-abc123.sh" {
		t.Errorf("TaskPaths.Script = %v, want %v", paths.Script, "/home/deploy/.launch/task-abc123.sh")
	}
	if paths.Output != "/home/deploy/.launch/task-abc123.log" {
		t.Errorf("TaskPaths.Output = %v, want %v", paths.Output, "/home/deploy/.launch/task-abc123.log")
	}
	if paths.ExitCode != "/home/deploy/.launch/task-abc123.exit" {
		t.Errorf("TaskPaths.ExitCode = %v, want %v", paths.ExitCode, "/home/deploy/.launch/task-abc123.exit")
	}
}

func TestGetTaskDir(t *testing.T) {
	want := "/home/deploy/.launch"
	if got := GetTaskDir("deploy"); got != want {
		t.Errorf("GetTaskDir() = %v, want %v", got, want)
	}
}

func TestHomeDir(t *testing.T) {
	tests := []struct {
		user string
		want string
	}{
		{"root", "/root"},
		{"deploy", "/home/deploy"},
	}

	for _, tt := range tests {
		if got := HomeDir(tt.user); got != tt.want {
			t.Errorf("HomeDir(%q) = %v, want %v", tt.user, got, tt.want)
		}
	}
}

func TestSSHDir(t *testing.T) {
	want := "/home/deploy/.ssh"
	if got := SSHDir("deploy"); got != want {
		t.Errorf("SSHDir() = %v, want %v", got, want)
	}
}

func TestAuthorizedKeysPath(t *testing.T) {
	want := "/home/deploy/.ssh/authorized_keys"
	if got := AuthorizedKeysPath("deploy"); got != want {
		t.Errorf("AuthorizedKeysPath() = %v, want %v", got, want)
	}
}

func TestJoinHome(t *testing.T) {
	want := "/home/deploy/.config/composer"
	if got := JoinHome("deploy", ".config", "composer"); got != want {
		t.Errorf("JoinHome() = %v, want %v", got, want)
	}
}

func TestFluentAPI(t *testing.T) {
	// Test the fluent API pattern
	site := ForSite("deploy", "example.com")

	// Access User methods through Site
	authKeys := site.User.AuthorizedKeys()
	wantAuthKeys := "/home/deploy/.ssh/authorized_keys"
	if authKeys != wantAuthKeys {
		t.Errorf("site.User.AuthorizedKeys() = %v, want %v", authKeys, wantAuthKeys)
	}

	// Build a release path
	releaseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	release := site.Release(releaseTime)
	wantRelease := "/home/deploy/example.com/releases/20240115120000"
	if release != wantRelease {
		t.Errorf("site.Release() = %v, want %v", release, wantRelease)
	}

	// Get env path
	env := site.Env()
	wantEnv := "/home/deploy/example.com/shared/.env"
	if env != wantEnv {
		t.Errorf("site.Env() = %v, want %v", env, wantEnv)
	}
}
