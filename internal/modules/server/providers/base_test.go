package providers

import (
	"encoding/json"
	"math"
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
		{"largest exact JSON integer", float64(1<<53 - 1), "9007199254740991"},
		{"float32 integer", float32(12345), "12345"},
		{"int", int(12345), "12345"},
		{"int32", int32(12345), "12345"},
		{"int64", int64(56516756), "56516756"},
		{"uint", uint(12345), "12345"},
		{"uint64", uint64(12345), "12345"},
		{"JSON number", json.Number("56516756"), "56516756"},
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

func TestExtractServerIDRejectsInvalidNumbersAndTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  any
	}{
		{"zero", float64(0)},
		{"negative", float64(-123)},
		{"fractional", 123.5},
		{"unsafe integer", float64(1 << 53)},
		{"NaN", math.NaN()},
		{"infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
		{"float32 fractional", float32(123.5)},
		{"float32 unsafe integer", float32(1 << 24)},
		{"float32 NaN", float32(math.NaN())},
		{"negative int", int(-1)},
		{"zero int32", int32(0)},
		{"negative int64", int64(-1)},
		{"zero uint", uint(0)},
		{"zero uint64", uint64(0)},
		{"fractional JSON number", json.Number("123.5")},
		{"negative JSON number", json.Number("-1")},
		{"zero JSON number", json.Number("0")},
		{"boolean", true},
		{"object", map[string]any{"id": 123}},
		{"array", []any{123}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, ExtractServerID(map[string]any{"id": tc.raw}, "id"))
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
