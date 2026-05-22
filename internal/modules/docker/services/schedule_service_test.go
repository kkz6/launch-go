package services

import "testing"

// TestLooksLikeCron — we only validate field count, not the per-field
// grammar. The host's cron daemon catches deeper errors. Pin the
// minimum so a refactor doesn't widen acceptance.
func TestLooksLikeCron(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"0 * * * *", true},
		{"*/5 * * * 1-5", true},
		{"   5  10  *  *  *   ", true}, // extra whitespace tolerated
		{"0 * * *", false},              // 4 fields
		{"0 * * * * *", false},          // 6 fields (we don't support seconds)
		{"", false},
		{"@daily", false}, // we deliberately don't accept @ shortcuts; the
		// host cron daemon may not either depending on distro.
	}
	for _, c := range cases {
		if got := looksLikeCron(c.in); got != c.want {
			t.Errorf("looksLikeCron(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
