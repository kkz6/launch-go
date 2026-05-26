package handlers

import "testing"

// slugifyForContainer mirrors tasks.SlugFromName — the two must agree
// or the WS handler will look up a container the deploy task never
// created. Pin the equivalence.
func TestSlugifyForContainer_MatchesTaskBehaviour(t *testing.T) {
	cases := []struct{ in, want string }{
		{"api", "api"},
		{"My API!", "my-api"},
		{"acme-prod", "acme-prod"},
		{"under_score", "under-score"},
		{"   spaces   ", "spaces"},
		{"foo/bar/baz", "foo-bar-baz"},
		{"trailing-", "trailing"},
		{"", "app"},
		{"!!!", "app"},
	}
	for _, c := range cases {
		if got := slugifyForContainer(c.in); got != c.want {
			t.Errorf("slugifyForContainer(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDatabaseContainerName_MirrorsTaskHelper(t *testing.T) {
	// Same shape as tasks.DatabaseContainerName — they must agree.
	got := databaseContainerName("Acme Prod", "orders-db")
	want := "launch-db-acme-prod-orders-db"
	if got != want {
		t.Errorf("databaseContainerName = %q, want %q", got, want)
	}
}

func TestComposeProjectName_MirrorsJobBehaviour(t *testing.T) {
	got := composeProjectName("Acme Prod", "monitoring stack")
	want := "acme-prod-monitoring-stack"
	if got != want {
		t.Errorf("composeProjectName = %q, want %q", got, want)
	}
}

func TestShellQuote_HandlesQuotes(t *testing.T) {
	if got := shellQuote("plain"); got != "'plain'" {
		t.Errorf("shellQuote(plain) = %q, want 'plain'", got)
	}
	if got := shellQuote("it's"); got != `'it'\''s'` {
		t.Errorf("shellQuote(it's) = %q, want 'it'\\''s'", got)
	}
	if got := shellQuote(""); got != "''" {
		t.Errorf("empty input should be empty quoted, got %q", got)
	}
}

func TestParseTail_ClampsToSafeRange(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 200},    // default
		{"abc", 200}, // non-numeric → default
		{"-5", 200},  // negative → default
		{"100", 100},
		{"99999", 5000}, // clamped to ceiling
	}
	for _, c := range cases {
		if got := parseTail(c.in); got != c.want {
			t.Errorf("parseTail(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
