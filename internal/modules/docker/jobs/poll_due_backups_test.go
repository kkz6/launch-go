package jobs

import (
	"testing"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
)

// TestCronDueInWindow pins the "is this expression due in [start, end)?"
// helper. This is the heart of the dokploy-style every-minute poller —
// regressions here mean either silent skipped backups or duplicate
// dispatch on boundary ticks. Worth pinning.
func TestCronDueInWindow(t *testing.T) {
	// Pick a moment that's NOT on a minute boundary so we can be precise
	// about what window the helper sees.
	base := time.Date(2026, 5, 23, 14, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		expr       string
		windowStart time.Time
		want       bool
	}{
		{
			name:        "every-minute fires every minute",
			expr:        "* * * * *",
			windowStart: base,
			want:        true,
		},
		{
			name:        "every-15-min fires at :00",
			expr:        "*/15 * * * *",
			windowStart: base, // 14:00
			want:        true,
		},
		{
			name:        "every-15-min does not fire at :01",
			expr:        "*/15 * * * *",
			windowStart: base.Add(time.Minute), // 14:01
			want:        false,
		},
		{
			name:        "every-15-min fires at :15",
			expr:        "*/15 * * * *",
			windowStart: base.Add(15 * time.Minute), // 14:15
			want:        true,
		},
		{
			name:        "hourly at :00 does not fire at :30",
			expr:        "0 * * * *",
			windowStart: base.Add(30 * time.Minute), // 14:30
			want:        false,
		},
		{
			name:        "daily at 02:00 does not fire at 14:00",
			expr:        "0 2 * * *",
			windowStart: base, // 14:00
			want:        false,
		},
		{
			name:        "daily at 14:00 fires at 14:00",
			expr:        "0 14 * * *",
			windowStart: base, // 14:00
			want:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cronDueInWindow(tc.expr, tc.windowStart, tc.windowStart.Add(time.Minute))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("cronDueInWindow(%q, %s) = %v, want %v",
					tc.expr, tc.windowStart.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

// TestCronDueInWindow_InvalidExpression confirms we return the parser's
// error rather than silently dropping bad cron strings. The poller logs
// + skips on error; testing the surface is enough.
func TestCronDueInWindow_InvalidExpression(t *testing.T) {
	_, err := cronDueInWindow("not a cron", time.Now(), time.Now().Add(time.Minute))
	if err == nil {
		t.Fatal("expected parse error for invalid cron, got nil")
	}
}

// TestParseRunMarkers walks the marker extraction. The script-side
// markers are a stable contract; regressions here mean failed backup
// runs aren't reporting object_key / size_bytes on the run row.
func TestParseRunMarkers(t *testing.T) {
	cases := []struct {
		name     string
		output   string
		wantKey  string
		wantSize int64
	}{
		{
			name: "both markers present",
			output: "starting...\n" +
				"::LAUNCH::size_bytes::1024\n" +
				"::LAUNCH::object_key::prefix/db/2026-05-23/abc.sql.gz\n" +
				"done\n",
			wantKey:  "prefix/db/2026-05-23/abc.sql.gz",
			wantSize: 1024,
		},
		{
			name:     "only size",
			output:   "::LAUNCH::size_bytes::500\n",
			wantKey:  "",
			wantSize: 500,
		},
		{
			name:     "no markers",
			output:   "ran cleanly with no markers",
			wantKey:  "",
			wantSize: 0,
		},
		{
			name:     "garbage after size is ignored",
			output:   "::LAUNCH::size_bytes::42abc\n",
			wantKey:  "",
			wantSize: 42,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotSize := tasks.ParseRunMarkers(tc.output)
			if gotKey != tc.wantKey {
				t.Errorf("object_key = %q, want %q", gotKey, tc.wantKey)
			}
			if gotSize != tc.wantSize {
				t.Errorf("size_bytes = %d, want %d", gotSize, tc.wantSize)
			}
		})
	}
}
