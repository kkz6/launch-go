package services

import "testing"

func TestValidateTraefikFilename(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		why     string
	}{
		// Happy path — what Launch writes itself + plausible user
		// custom names.
		{"middlewares.yml", false, "exact name Launch ships out of the box"},
		{"acme-prod-api.yml", false, "auto-generated per-app config"},
		{"custom-headers.yaml", false, "user custom — .yaml also accepted"},
		{"rate.limit.yml", false, "dots inside filename are fine"},

		// Path traversal — every variant must fail.
		{"../traefik.yml", true, "leading .. would escape dynamic/"},
		{"sub/../middlewares.yml", true, "embedded .. in path"},
		{"../../etc/passwd.yml", true, "obvious traversal attempt"},
		{"a/b.yml", true, "slash in name"},
		{`a\b.yml`, true, "backslash in name"},

		// Hidden / weird filenames.
		{".hidden.yml", true, "leading dot disallowed (no hidden files)"},
		{"", true, "empty filename"},
		{"   ", true, "whitespace only"},
		{"middlewares", true, "no extension"},
		{"middlewares.json", true, "wrong extension"},
		{"middlewares.txt", true, "wrong extension"},

		// Reserved names — these are Launch-managed and must not be
		// writable through this endpoint.
		{"acme.json", true, "reserved (extension also wrong but the reservation check fires first)"},
		{"access.log", true, "reserved"},
		{"access.log.tmp", true, "reserved"},

		// Length boundary — regex caps the body at 1 + 75 = 76 chars
		// then .yml/.yaml. Max total length is therefore 76 + 4 = 80
		// for .yml or 76 + 5 = 81 for .yaml.
		{repeatChar("x", 76) + ".yml", false, "exactly at the 80-char cap with .yml"},
		{repeatChar("x", 77) + ".yml", true, "81 chars — one over the .yml cap"},
		{repeatChar("x", 100) + ".yml", true, "way over cap"},
	}
	for _, c := range cases {
		err := validateTraefikFilename(c.in)
		got := err != nil
		if got != c.wantErr {
			t.Errorf("validateTraefikFilename(%q) error=%v (want err=%v) — %s",
				c.in, err, c.wantErr, c.why)
		}
	}
}

func repeatChar(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
