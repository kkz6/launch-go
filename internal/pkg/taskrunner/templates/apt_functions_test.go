package templates

import (
	"regexp"
	"testing"
)

// Pulls the pattern out of the rendered bash so the table below exercises the
// exact expression that ships to servers, not a copy that can drift from it.
func pgrepPattern(t *testing.T) *regexp.Regexp {
	t.Helper()

	matches := regexp.MustCompile(`APT_PROCESS_PATTERN='([^']+)'`).FindStringSubmatch(AptFunctions())
	if matches == nil {
		t.Fatal("AptFunctions no longer defines APT_PROCESS_PATTERN as a single-quoted pattern")
	}

	pattern, err := regexp.Compile(matches[1])
	if err != nil {
		t.Fatalf("pgrep pattern does not compile: %v", err)
	}

	return pattern
}

// Anything matching a package-manager name as a mere substring wedges
// waitForAptUnlock for as long as that process lives.
func TestWaitForAptUnlockMatchesOnlyRealPackageManagers(t *testing.T) {
	pattern := pgrepPattern(t)

	cases := []struct {
		name    string
		cmdline string
		want    bool
	}{
		{
			// Runs permanently on a stock Ubuntu box, so matching it means
			// waitForAptUnlock can never return.
			name:    "unattended-upgrades shutdown watcher",
			cmdline: "/usr/bin/python3 /usr/share/unattended-upgrades/unattended-upgrade-shutdown --wait-for-signal",
			want:    false,
		},
		{
			name:    "apt download method",
			cmdline: "/usr/lib/apt/methods/https",
			want:    false,
		},
		{
			name:    "our own task script",
			cmdline: "bash /home/launch/.launch/tasks/01JQ/script.sh",
			want:    false,
		},
		{
			name:    "unattended-upgrade actually upgrading",
			cmdline: "/usr/bin/python3 /usr/bin/unattended-upgrade --download-only",
			want:    true,
		},
		{
			name:    "apt-daily timer job",
			cmdline: "/bin/sh /usr/lib/apt/apt.systemd.daily update",
			want:    true,
		},
		{
			name:    "apt-get installing",
			cmdline: "apt-get -y install php8.3-cli",
			want:    true,
		},
		{
			name:    "dpkg mid-configure",
			cmdline: "/usr/bin/dpkg --configure -a",
			want:    true,
		},
		{
			name:    "bare dpkg",
			cmdline: "/usr/bin/dpkg",
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pattern.MatchString(tc.cmdline); got != tc.want {
				t.Errorf("match(%q) = %v, want %v", tc.cmdline, got, tc.want)
			}
		})
	}
}

// An unbounded wait turns a stuck daemon into a silent task-level timeout
// whose only output is the same line repeated hundreds of times.
func TestWaitForAptUnlockIsBounded(t *testing.T) {
	apt := AptFunctions()

	for _, needle := range []string{"APT_WAIT_TIMEOUT_SECONDS", "Giving up"} {
		if !regexp.MustCompile(regexp.QuoteMeta(needle)).MatchString(apt) {
			t.Errorf("waitForAptUnlock is missing %q — the wait must be bounded and explain why it gave up", needle)
		}
	}
}
