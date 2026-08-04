package markers

import "testing"

func TestErrorAndExitCodeFormatters(t *testing.T) {
	t.Parallel()

	if got, want := FormatError("disk full"), "::LAUNCH::error::disk full"; got != want {
		t.Fatalf("FormatError() = %q, want %q", got, want)
	}
	if got, want := FormatExitCode(23), "::LAUNCH::exit_code::23"; got != want {
		t.Fatalf("FormatExitCode() = %q, want %q", got, want)
	}
}
