package status

import "testing"

func TestParseVersion(t *testing.T) {
	cases := map[string]string{
		// Real `--version` output shapes from each tool we probe.
		"launch-agent version 0.8.0":                                       "0.8.0",
		"Docker version 27.3.1, build 41ca978":                             "27.3.1",
		"Version: 3.1.2\nCodename: charmedmilkshake\nGo version: go1.22.4": "3.1.2",
		"4.2.5": "4.2.5",
		"Redis server v=7.0.11 sha=00000000:0 malloc=jemalloc":    "7.0.11",
		"/usr/sbin/mysqld  Ver 8.0.36-0ubuntu0.22.04.1 for Linux": "8.0.36",
		"psql (PostgreSQL) 16.2":                                  "16.2",
		"v2.7.6 h1:abcd":                                          "2.7.6",
		"Composer version 2.7.1 2024-02-09 15:26:28":              "2.7.1",
		"v21.7.3": "21.7.3",
		"1.1.20":  "1.1.20",
		"PHP 8.2.18 (cli) (built: Mar 14 2024 17:01:00)": "8.2.18",
		"":                "",
		"no version here": "",
	}
	for in, want := range cases {
		if got := ParseVersion(in); got != want {
			t.Errorf("ParseVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	cases := map[string]string{
		"launch_agent": "launch-agent --version 2>/dev/null",
		"docker":       "docker --version 2>/dev/null",
		"traefik":      "docker exec launch-traefik traefik version 2>/dev/null",
		"supervisor":   "supervisord --version 2>/dev/null",
		"redis":        "redis-server --version 2>/dev/null",
		"php82":        "php8.2 --version 2>/dev/null",
		"php74":        "php7.4 --version 2>/dev/null",
		"unknown_xyz":  "",
	}
	for sw, want := range cases {
		if got := VersionCommand(sw); got != want {
			t.Errorf("VersionCommand(%q) = %q, want %q", sw, got, want)
		}
	}
}
