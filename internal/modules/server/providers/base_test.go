package providers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractServerID_PreservesIntegerForm pins the fix for the bug where
// DO's `id` (a JSON integer) came back as a float64 after Unmarshal, and
// fmt.Sprintf("%v", float64(56516756)) produced "5.6516756e+07" — which
// DigitalOcean's droplet-create endpoint rejected with
//
//	"5.6516756e+07 are invalid key identifiers for Droplet creation"
//
// The fix type-switches on the value so large integer IDs round-trip as
// plain decimal strings.
func TestExtractServerID_PreservesIntegerForm(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want string
	}{
		{"small integer as float64", float64(12), "12"},
		{"DO ssh-key sized integer", float64(56516756), "56516756"},
		{"int", int(12345), "12345"},
		{"int64", int64(56516756), "56516756"},
		{"string fingerprint", "aa:bb:cc:dd", "aa:bb:cc:dd"},
		{"nil missing key", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{"id": tc.raw}
			got := ExtractServerID(data, "id")
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestExtractServerID_JSONRoundTrip exercises the actual path that caused
// the bug: JSON-decoded into an interface{} map. Without this, regressions
// where we re-introduce fmt.Sprintf("%v", ...) for IDs slip through.
func TestExtractServerID_JSONRoundTrip(t *testing.T) {
	var data map[string]any
	err := json.Unmarshal([]byte(`{"id": 56516756}`), &data)
	assert.NoError(t, err)
	assert.Equal(t, "56516756", ExtractServerID(data, "id"))
}
