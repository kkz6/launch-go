package serverpath

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
	want := "/home/deploy/.launch-tasks"
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

func TestCaddyConfig(t *testing.T) {
	want := "/etc/caddy/Caddyfile"
	if got := CaddyConfig(); got != want {
		t.Errorf("CaddyConfig() = %v, want %v", got, want)
	}
}

func TestCaddySites(t *testing.T) {
	want := "/etc/caddy/sites"
	if got := CaddySites(); got != want {
		t.Errorf("CaddySites() = %v, want %v", got, want)
	}
}

func TestCaddySiteConfig(t *testing.T) {
	want := "/etc/caddy/sites/example.com"
	if got := CaddySiteConfig("example.com"); got != want {
		t.Errorf("CaddySiteConfig() = %v, want %v", got, want)
	}
}

func TestPHPFPMPool(t *testing.T) {
	want := "/etc/php/8.2/fpm/pool.d"
	if got := PHPFPMPool("8.2"); got != want {
		t.Errorf("PHPFPMPool() = %v, want %v", got, want)
	}
}

func TestPHPFPMConfig(t *testing.T) {
	want := "/etc/php/8.2/fpm/php-fpm.conf"
	if got := PHPFPMConfig("8.2"); got != want {
		t.Errorf("PHPFPMConfig() = %v, want %v", got, want)
	}
}

func TestPHPIni(t *testing.T) {
	want := "/etc/php/8.2/fpm/php.ini"
	if got := PHPIni("8.2"); got != want {
		t.Errorf("PHPIni() = %v, want %v", got, want)
	}
}

func TestPHPCLIIni(t *testing.T) {
	want := "/etc/php/8.2/cli/php.ini"
	if got := PHPCLIIni("8.2"); got != want {
		t.Errorf("PHPCLIIni() = %v, want %v", got, want)
	}
}

func TestPHPBinary(t *testing.T) {
	want := "/usr/bin/php8.2"
	if got := PHPBinary("8.2"); got != want {
		t.Errorf("PHPBinary() = %v, want %v", got, want)
	}
}

func TestSupervisorConfig(t *testing.T) {
	want := "/etc/supervisor/conf.d"
	if got := SupervisorConfig(); got != want {
		t.Errorf("SupervisorConfig() = %v, want %v", got, want)
	}
}

func TestSupervisorSiteConfig(t *testing.T) {
	want := "/etc/supervisor/conf.d/example-worker.conf"
	if got := SupervisorSiteConfig("example-worker"); got != want {
		t.Errorf("SupervisorSiteConfig() = %v, want %v", got, want)
	}
}

func TestNginxSites(t *testing.T) {
	want := "/etc/nginx/sites-available"
	if got := NginxSites(); got != want {
		t.Errorf("NginxSites() = %v, want %v", got, want)
	}
}

func TestNginxSitesEnabled(t *testing.T) {
	want := "/etc/nginx/sites-enabled"
	if got := NginxSitesEnabled(); got != want {
		t.Errorf("NginxSitesEnabled() = %v, want %v", got, want)
	}
}

func TestNginxSiteConfig(t *testing.T) {
	want := "/etc/nginx/sites-available/example.com"
	if got := NginxSiteConfig("example.com"); got != want {
		t.Errorf("NginxSiteConfig() = %v, want %v", got, want)
	}
}

func TestMySQLConfig(t *testing.T) {
	want := "/etc/mysql/mysql.conf.d/mysqld.cnf"
	if got := MySQLConfig(); got != want {
		t.Errorf("MySQLConfig() = %v, want %v", got, want)
	}
}

func TestPostgreSQLConfig(t *testing.T) {
	want := "/etc/postgresql/15/main/postgresql.conf"
	if got := PostgreSQLConfig("15"); got != want {
		t.Errorf("PostgreSQLConfig() = %v, want %v", got, want)
	}
}

func TestPostgreSQLHBA(t *testing.T) {
	want := "/etc/postgresql/15/main/pg_hba.conf"
	if got := PostgreSQLHBA("15"); got != want {
		t.Errorf("PostgreSQLHBA() = %v, want %v", got, want)
	}
}

func TestCronD(t *testing.T) {
	want := "/etc/cron.d"
	if got := CronD(); got != want {
		t.Errorf("CronD() = %v, want %v", got, want)
	}
}

func TestCronJob(t *testing.T) {
	want := "/etc/cron.d/mysite-scheduler"
	if got := CronJob("mysite-scheduler"); got != want {
		t.Errorf("CronJob() = %v, want %v", got, want)
	}
}

func TestUFWRules(t *testing.T) {
	want := "/etc/ufw/user.rules"
	if got := UFWRules(); got != want {
		t.Errorf("UFWRules() = %v, want %v", got, want)
	}
}

func TestUFWRulesV6(t *testing.T) {
	want := "/etc/ufw/user6.rules"
	if got := UFWRulesV6(); got != want {
		t.Errorf("UFWRulesV6() = %v, want %v", got, want)
	}
}

func TestSSLCertificate(t *testing.T) {
	want := "/etc/letsencrypt/live/example.com/fullchain.pem"
	if got := SSLCertificate("example.com"); got != want {
		t.Errorf("SSLCertificate() = %v, want %v", got, want)
	}
}

func TestSSLPrivateKey(t *testing.T) {
	want := "/etc/letsencrypt/live/example.com/privkey.pem"
	if got := SSLPrivateKey("example.com"); got != want {
		t.Errorf("SSLPrivateKey() = %v, want %v", got, want)
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
