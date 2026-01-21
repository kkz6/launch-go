package paths

import "testing"

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
