package markers

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantNil  bool
		wantType string
		wantVal  string
	}{
		{
			name:     "step completed marker",
			line:     "::LAUNCH::step_completed::configure_swap",
			wantType: StepCompleted,
			wantVal:  "configure_swap",
		},
		{
			name:     "software installed marker",
			line:     "::LAUNCH::software_installed::php83",
			wantType: SoftwareInstalled,
			wantVal:  "php83",
		},
		{
			name:     "progress marker",
			line:     "::LAUNCH::progress::50",
			wantType: Progress,
			wantVal:  "50",
		},
		{
			name:     "status marker",
			line:     "::LAUNCH::status::Installing dependencies",
			wantType: Status,
			wantVal:  "Installing dependencies",
		},
		{
			name:     "error marker",
			line:     "::LAUNCH::error::Failed to download package",
			wantType: Error,
			wantVal:  "Failed to download package",
		},
		{
			name:     "exit code marker",
			line:     "::LAUNCH::exit_code::0",
			wantType: ExitCode,
			wantVal:  "0",
		},
		{
			name:     "with leading whitespace",
			line:     "  ::LAUNCH::progress::25",
			wantType: Progress,
			wantVal:  "25",
		},
		{
			name:     "with trailing whitespace",
			line:     "::LAUNCH::progress::75  ",
			wantType: Progress,
			wantVal:  "75",
		},
		{
			name:    "not a marker - regular output",
			line:    "Installing PHP 8.3...",
			wantNil: true,
		},
		{
			name:    "not a marker - partial prefix",
			line:    "::LAUNCH::incomplete",
			wantNil: true,
		},
		{
			name:    "not a marker - empty line",
			line:    "",
			wantNil: true,
		},
		{
			name:    "not a marker - just prefix",
			line:    "::LAUNCH::",
			wantNil: true,
		},
		{
			name:     "value with colons",
			line:     "::LAUNCH::status::Step 1: Configure system",
			wantType: Status,
			wantVal:  "Step 1: Configure system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.line)

			if tt.wantNil {
				if got != nil {
					t.Errorf("Parse() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("Parse() = nil, want marker")
			}

			if got.Type != tt.wantType {
				t.Errorf("Parse().Type = %q, want %q", got.Type, tt.wantType)
			}

			if got.Value != tt.wantVal {
				t.Errorf("Parse().Value = %q, want %q", got.Value, tt.wantVal)
			}
		})
	}
}

func TestIsMarker(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"::LAUNCH::progress::50", true},
		{"Some text ::LAUNCH::status::message", true},
		{"Regular output", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := IsMarker(tt.line); got != tt.want {
				t.Errorf("IsMarker(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestMarker_ProgressValue(t *testing.T) {
	tests := []struct {
		name   string
		marker *Marker
		want   int
	}{
		{
			name:   "valid progress",
			marker: &Marker{Type: Progress, Value: "50"},
			want:   50,
		},
		{
			name:   "zero progress",
			marker: &Marker{Type: Progress, Value: "0"},
			want:   0,
		},
		{
			name:   "100 progress",
			marker: &Marker{Type: Progress, Value: "100"},
			want:   100,
		},
		{
			name:   "over 100 clamped",
			marker: &Marker{Type: Progress, Value: "150"},
			want:   100,
		},
		{
			name:   "negative clamped to 0",
			marker: &Marker{Type: Progress, Value: "-10"},
			want:   0,
		},
		{
			name:   "invalid value",
			marker: &Marker{Type: Progress, Value: "abc"},
			want:   0,
		},
		{
			name:   "wrong type",
			marker: &Marker{Type: Status, Value: "50"},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.marker.ProgressValue(); got != tt.want {
				t.Errorf("ProgressValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarker_ExitCodeValue(t *testing.T) {
	tests := []struct {
		name   string
		marker *Marker
		want   int
	}{
		{
			name:   "success",
			marker: &Marker{Type: ExitCode, Value: "0"},
			want:   0,
		},
		{
			name:   "error code",
			marker: &Marker{Type: ExitCode, Value: "1"},
			want:   1,
		},
		{
			name:   "signal exit",
			marker: &Marker{Type: ExitCode, Value: "141"},
			want:   141,
		},
		{
			name:   "invalid value",
			marker: &Marker{Type: ExitCode, Value: "abc"},
			want:   -1,
		},
		{
			name:   "wrong type",
			marker: &Marker{Type: Progress, Value: "0"},
			want:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.marker.ExitCodeValue(); got != tt.want {
				t.Errorf("ExitCodeValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		markerType string
		value      string
		want       string
	}{
		{Progress, "50", "::LAUNCH::progress::50"},
		{StepCompleted, "configure_swap", "::LAUNCH::step_completed::configure_swap"},
		{Status, "Installing", "::LAUNCH::status::Installing"},
	}

	for _, tt := range tests {
		t.Run(tt.markerType, func(t *testing.T) {
			if got := Format(tt.markerType, tt.value); got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBashEcho(t *testing.T) {
	got := BashEcho(Progress, "50")
	want := `echo "::LAUNCH::progress::50"`

	if got != want {
		t.Errorf("BashEcho() = %q, want %q", got, want)
	}
}

func TestBashEchoHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "progress",
			got:  BashEchoProgress(50),
			want: `echo "::LAUNCH::progress::50"`,
		},
		{
			name: "step completed",
			got:  BashEchoStepCompleted("configure_swap"),
			want: `echo "::LAUNCH::step_completed::configure_swap"`,
		},
		{
			name: "software installed",
			got:  BashEchoSoftwareInstalled("php83"),
			want: `echo "::LAUNCH::software_installed::php83"`,
		},
		{
			name: "status",
			got:  BashEchoStatus("Installing packages"),
			want: `echo "::LAUNCH::status::Installing packages"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
