package enumtypes

import "testing"

type color string

const (
	colorRed   color = "red"
	colorGreen color = "green"
	colorBlue  color = "blue"
)

var colorLabels = map[color]string{
	colorRed:   "Red",
	colorGreen: "Green",
	colorBlue:  "Blue",
}

func TestLabel_Found(t *testing.T) {
	tests := []struct {
		val  color
		want string
	}{
		{colorRed, "Red"},
		{colorGreen, "Green"},
		{colorBlue, "Blue"},
	}

	for _, tt := range tests {
		t.Run(string(tt.val), func(t *testing.T) {
			if got := Label(tt.val, colorLabels, "Unknown"); got != tt.want {
				t.Errorf("Label(%q) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}

func TestLabel_FallbackUsedForUnknown(t *testing.T) {
	if got := Label(color("magenta"), colorLabels, "Unknown"); got != "Unknown" {
		t.Errorf("Label(magenta) = %q, want %q", got, "Unknown")
	}
}

func TestLabel_FallbackEmptyStringIsRespected(t *testing.T) {
	// Some enums prefer "" over a placeholder; make sure the helper doesn't
	// substitute its own default.
	if got := Label(color("magenta"), colorLabels, ""); got != "" {
		t.Errorf("Label(magenta, empty fallback) = %q, want \"\"", got)
	}
}
