package models

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
)

func TestJSONMap_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    JSONMap
		wantErr bool
	}{
		{
			name:    "nil value",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "empty bytes",
			input:   []byte{},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "valid JSON bytes",
			input:   []byte(`{"key":"value","num":42}`),
			want:    JSONMap{"key": "value", "num": float64(42)},
			wantErr: false,
		},
		{
			name:    "valid JSON string",
			input:   `{"key":"value"}`,
			want:    JSONMap{"key": "value"},
			wantErr: false,
		},
		{
			name:    "array JSON is treated as nil",
			input:   []byte(`["item1","item2"]`),
			want:    nil,
			wantErr: false,
		},
		{
			name:    "array with whitespace",
			input:   []byte(`  ["item1"]`),
			want:    nil,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   []byte(`{invalid}`),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   123,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nested object",
			input:   []byte(`{"nested":{"inner":"value"}}`),
			want:    JSONMap{"nested": map[string]any{"inner": "value"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONMap
			err := j.Scan(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSONMap.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.want == nil && j != nil {
					t.Errorf("JSONMap.Scan() = %v, want nil", j)
				} else if tt.want != nil {
					got, _ := json.Marshal(j)
					want, _ := json.Marshal(tt.want)
					if string(got) != string(want) {
						t.Errorf("JSONMap.Scan() = %s, want %s", got, want)
					}
				}
			}
		})
	}
}

func TestJSONMap_Value(t *testing.T) {
	tests := []struct {
		name    string
		j       JSONMap
		want    string
		wantNil bool
	}{
		{
			name:    "nil map",
			j:       nil,
			wantNil: true,
		},
		{
			name: "simple map",
			j:    JSONMap{"key": "value"},
			want: `{"key":"value"}`,
		},
		{
			name: "map with multiple types",
			j:    JSONMap{"str": "value", "num": 42, "bool": true},
			want: `{"bool":true,"num":42,"str":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.j.Value()
			if err != nil {
				t.Errorf("JSONMap.Value() error = %v", err)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("JSONMap.Value() = %v, want nil", got)
				}
				return
			}

			gotBytes, ok := got.([]byte)
			if !ok {
				t.Errorf("JSONMap.Value() returned %T, want []byte", got)
				return
			}

			var gotMap, wantMap map[string]any
			json.Unmarshal(gotBytes, &gotMap)
			json.Unmarshal([]byte(tt.want), &wantMap)

			gotJSON, _ := json.Marshal(gotMap)
			wantJSON, _ := json.Marshal(wantMap)

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("JSONMap.Value() = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestJSONMap_GetString(t *testing.T) {
	j := JSONMap{"str": "value", "num": 42, "bool": true}

	if got := j.GetString("str"); got != "value" {
		t.Errorf("GetString('str') = %q, want %q", got, "value")
	}

	if got := j.GetString("num"); got != "" {
		t.Errorf("GetString('num') = %q, want empty string", got)
	}

	if got := j.GetString("missing"); got != "" {
		t.Errorf("GetString('missing') = %q, want empty string", got)
	}

	var nilMap JSONMap
	if got := nilMap.GetString("key"); got != "" {
		t.Errorf("nil map GetString() = %q, want empty string", got)
	}
}

func TestJSONMap_GetInt(t *testing.T) {
	j := JSONMap{"float": float64(42), "int": 10, "str": "value"}

	if got := j.GetInt("float"); got != 42 {
		t.Errorf("GetInt('float') = %d, want 42", got)
	}

	if got := j.GetInt("str"); got != 0 {
		t.Errorf("GetInt('str') = %d, want 0", got)
	}

	if got := j.GetInt("missing"); got != 0 {
		t.Errorf("GetInt('missing') = %d, want 0", got)
	}

	var nilMap JSONMap
	if got := nilMap.GetInt("key"); got != 0 {
		t.Errorf("nil map GetInt() = %d, want 0", got)
	}
}

func TestJSONMap_GetInt64(t *testing.T) {
	j := JSONMap{"float": float64(9223372036854775807), "int64": int64(100)}

	if got := j.GetInt64("float"); got != 9223372036854775807 {
		t.Errorf("GetInt64('float') = %d, want 9223372036854775807", got)
	}

	if got := j.GetInt64("int64"); got != 100 {
		t.Errorf("GetInt64('int64') = %d, want 100", got)
	}

	var nilMap JSONMap
	if got := nilMap.GetInt64("key"); got != 0 {
		t.Errorf("nil map GetInt64() = %d, want 0", got)
	}
}

func TestJSONMap_GetFloat64(t *testing.T) {
	j := JSONMap{"float": float64(3.14), "str": "value"}

	if got := j.GetFloat64("float"); got != 3.14 {
		t.Errorf("GetFloat64('float') = %f, want 3.14", got)
	}

	if got := j.GetFloat64("str"); got != 0 {
		t.Errorf("GetFloat64('str') = %f, want 0", got)
	}

	var nilMap JSONMap
	if got := nilMap.GetFloat64("key"); got != 0 {
		t.Errorf("nil map GetFloat64() = %f, want 0", got)
	}
}

func TestJSONMap_GetBool(t *testing.T) {
	j := JSONMap{"true": true, "false": false, "str": "value"}

	if got := j.GetBool("true"); !got {
		t.Errorf("GetBool('true') = %v, want true", got)
	}

	if got := j.GetBool("false"); got {
		t.Errorf("GetBool('false') = %v, want false", got)
	}

	if got := j.GetBool("str"); got {
		t.Errorf("GetBool('str') = %v, want false", got)
	}

	var nilMap JSONMap
	if got := nilMap.GetBool("key"); got {
		t.Errorf("nil map GetBool() = %v, want false", got)
	}
}

func TestJSONMap_Has(t *testing.T) {
	j := JSONMap{"key": "value", "nil_value": nil}

	if !j.Has("key") {
		t.Error("Has('key') = false, want true")
	}

	if !j.Has("nil_value") {
		t.Error("Has('nil_value') = false, want true")
	}

	if j.Has("missing") {
		t.Error("Has('missing') = true, want false")
	}

	var nilMap JSONMap
	if nilMap.Has("key") {
		t.Error("nil map Has() = true, want false")
	}
}

func TestJSONMap_GetMap(t *testing.T) {
	j := JSONMap{
		"nested": map[string]any{"inner": "value"},
		"str":    "value",
	}

	nested := j.GetMap("nested")
	if nested == nil {
		t.Error("GetMap('nested') = nil, want map")
	} else if nested.GetString("inner") != "value" {
		t.Errorf("GetMap('nested')['inner'] = %q, want 'value'", nested.GetString("inner"))
	}

	if got := j.GetMap("str"); got != nil {
		t.Errorf("GetMap('str') = %v, want nil", got)
	}

	var nilMap JSONMap
	if got := nilMap.GetMap("key"); got != nil {
		t.Errorf("nil map GetMap() = %v, want nil", got)
	}
}

func TestJSONMap_GetStringSlice(t *testing.T) {
	j := JSONMap{
		"tags":  []any{"tag1", "tag2", "tag3"},
		"mixed": []any{"str", 123},
		"str":   "value",
	}

	tags := j.GetStringSlice("tags")
	if len(tags) != 3 {
		t.Errorf("GetStringSlice('tags') len = %d, want 3", len(tags))
	} else if tags[0] != "tag1" || tags[1] != "tag2" || tags[2] != "tag3" {
		t.Errorf("GetStringSlice('tags') = %v, want [tag1, tag2, tag3]", tags)
	}

	mixed := j.GetStringSlice("mixed")
	if len(mixed) != 1 || mixed[0] != "str" {
		t.Errorf("GetStringSlice('mixed') = %v, want [str]", mixed)
	}

	if got := j.GetStringSlice("str"); got != nil {
		t.Errorf("GetStringSlice('str') = %v, want nil", got)
	}

	var nilMap JSONMap
	if got := nilMap.GetStringSlice("key"); got != nil {
		t.Errorf("nil map GetStringSlice() = %v, want nil", got)
	}
}

func TestJSONStringMap_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    JSONStringMap
		wantErr bool
	}{
		{
			name:    "nil value",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "valid JSON",
			input:   []byte(`{"key":"value","another":"data"}`),
			want:    JSONStringMap{"key": "value", "another": "data"},
			wantErr: false,
		},
		{
			name:    "string input",
			input:   `{"key":"value"}`,
			want:    JSONStringMap{"key": "value"},
			wantErr: false,
		},
		{
			name:    "empty bytes",
			input:   []byte{},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONStringMap
			err := j.Scan(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSONStringMap.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.want == nil && j != nil {
					t.Errorf("JSONStringMap.Scan() = %v, want nil", j)
				} else if tt.want != nil {
					for k, v := range tt.want {
						if j[k] != v {
							t.Errorf("JSONStringMap.Scan()[%q] = %q, want %q", k, j[k], v)
						}
					}
				}
			}
		})
	}
}

func TestJSONStringMap_Value(t *testing.T) {
	tests := []struct {
		name    string
		j       JSONStringMap
		wantNil bool
	}{
		{
			name:    "nil map",
			j:       nil,
			wantNil: true,
		},
		{
			name:    "with values",
			j:       JSONStringMap{"key": "value"},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.j.Value()
			if err != nil {
				t.Errorf("JSONStringMap.Value() error = %v", err)
				return
			}

			if tt.wantNil && got != nil {
				t.Errorf("JSONStringMap.Value() = %v, want nil", got)
			}

			if !tt.wantNil && got == nil {
				t.Errorf("JSONStringMap.Value() = nil, want non-nil")
			}
		})
	}
}

func TestJSONStringMap_Get(t *testing.T) {
	j := JSONStringMap{"key": "value"}

	if got := j.Get("key", "default"); got != "value" {
		t.Errorf("Get('key', 'default') = %q, want 'value'", got)
	}

	if got := j.Get("missing", "default"); got != "default" {
		t.Errorf("Get('missing', 'default') = %q, want 'default'", got)
	}

	var nilMap JSONStringMap
	if got := nilMap.Get("key", "default"); got != "default" {
		t.Errorf("nil map Get() = %q, want 'default'", got)
	}
}

func TestJSONStringMap_Has(t *testing.T) {
	j := JSONStringMap{"key": "value"}

	if !j.Has("key") {
		t.Error("Has('key') = false, want true")
	}

	if j.Has("missing") {
		t.Error("Has('missing') = true, want false")
	}

	var nilMap JSONStringMap
	if nilMap.Has("key") {
		t.Error("nil map Has() = true, want false")
	}
}

func TestJSONSlice_Scan(t *testing.T) {
	t.Run("string slice", func(t *testing.T) {
		var j JSONSlice[string]
		err := j.Scan([]byte(`["a","b","c"]`))
		if err != nil {
			t.Errorf("Scan() error = %v", err)
		}

		if len(j) != 3 || j[0] != "a" || j[1] != "b" || j[2] != "c" {
			t.Errorf("Scan() = %v, want [a, b, c]", j)
		}
	})

	t.Run("int slice", func(t *testing.T) {
		var j JSONSlice[int]
		err := j.Scan([]byte(`[1,2,3]`))
		if err != nil {
			t.Errorf("Scan() error = %v", err)
		}

		if len(j) != 3 || j[0] != 1 || j[1] != 2 || j[2] != 3 {
			t.Errorf("Scan() = %v, want [1, 2, 3]", j)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		var j JSONSlice[string]
		err := j.Scan(nil)
		if err != nil {
			t.Errorf("Scan() error = %v", err)
		}

		if j != nil {
			t.Errorf("Scan(nil) = %v, want nil", j)
		}
	})

	t.Run("string input", func(t *testing.T) {
		var j JSONSlice[string]
		err := j.Scan(`["x","y"]`)
		if err != nil {
			t.Errorf("Scan() error = %v", err)
		}

		if len(j) != 2 {
			t.Errorf("Scan() len = %d, want 2", len(j))
		}
	})

	t.Run("empty bytes", func(t *testing.T) {
		var j JSONSlice[string]
		err := j.Scan([]byte{})
		if err != nil {
			t.Errorf("Scan() error = %v", err)
		}

		if j != nil {
			t.Errorf("Scan([]) = %v, want nil", j)
		}
	})
}

func TestJSONSlice_Value(t *testing.T) {
	t.Run("string slice", func(t *testing.T) {
		j := JSONSlice[string]{"a", "b", "c"}
		got, err := j.Value()
		if err != nil {
			t.Errorf("Value() error = %v", err)
		}

		gotBytes, ok := got.([]byte)
		if !ok {
			t.Errorf("Value() returned %T, want []byte", got)
		}

		if string(gotBytes) != `["a","b","c"]` {
			t.Errorf("Value() = %s, want [\"a\",\"b\",\"c\"]", gotBytes)
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		var j JSONSlice[string]
		got, err := j.Value()
		if err != nil {
			t.Errorf("Value() error = %v", err)
		}

		if got != nil {
			t.Errorf("Value() = %v, want nil", got)
		}
	})
}

func TestJSONStringSlice(t *testing.T) {
	// JSONStringSlice is an alias for JSONSlice[string]
	var j JSONStringSlice
	err := j.Scan([]byte(`["step1","step2"]`))
	if err != nil {
		t.Errorf("Scan() error = %v", err)
	}

	if len(j) != 2 || j[0] != "step1" || j[1] != "step2" {
		t.Errorf("JSONStringSlice = %v, want [step1, step2]", j)
	}
}

func TestJSONArray_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantLen int
		wantErr bool
	}{
		{
			name:    "nil value",
			input:   nil,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "valid array",
			input:   []byte(`["string",42,true,null]`),
			wantLen: 4,
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   []byte(`[]`),
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "string input",
			input:   `["a","b"]`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "empty bytes",
			input:   []byte{},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONArray
			err := j.Scan(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSONArray.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantLen > 0 && len(j) != tt.wantLen {
				t.Errorf("JSONArray.Scan() len = %d, want %d", len(j), tt.wantLen)
			}
		})
	}
}

func TestJSONArray_Value(t *testing.T) {
	tests := []struct {
		name    string
		j       JSONArray
		want    string
		wantNil bool
	}{
		{
			name:    "nil array",
			j:       nil,
			wantNil: true,
		},
		{
			name: "mixed array",
			j:    JSONArray{"string", float64(42), true},
			want: `["string",42,true]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.j.Value()
			if err != nil {
				t.Errorf("JSONArray.Value() error = %v", err)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("JSONArray.Value() = %v, want nil", got)
				}
				return
			}

			gotBytes, ok := got.([]byte)
			if !ok {
				t.Errorf("JSONArray.Value() returned %T, want []byte", got)
				return
			}

			if string(gotBytes) != tt.want {
				t.Errorf("JSONArray.Value() = %s, want %s", gotBytes, tt.want)
			}
		})
	}
}

// Test that types implement the required interfaces
func TestInterfaceImplementations(t *testing.T) {
	var _ driver.Valuer = JSONMap{}
	var _ driver.Valuer = JSONStringMap{}
	var _ driver.Valuer = JSONSlice[string]{}
	var _ driver.Valuer = JSONArray{}

	// Scanner is tested implicitly through Scan tests
}

// Test roundtrip serialization
func TestRoundtrip(t *testing.T) {
	t.Run("JSONMap roundtrip", func(t *testing.T) {
		original := JSONMap{
			"string": "value",
			"number": float64(42),
			"bool":   true,
			"nested": map[string]any{"inner": "data"},
		}

		value, err := original.Value()
		if err != nil {
			t.Fatalf("Value() error = %v", err)
		}

		var restored JSONMap
		err = restored.Scan(value)
		if err != nil {
			t.Fatalf("Scan() error = %v", err)
		}

		if restored.GetString("string") != "value" {
			t.Errorf("roundtrip failed for string")
		}

		if restored.GetInt("number") != 42 {
			t.Errorf("roundtrip failed for number")
		}

		if !restored.GetBool("bool") {
			t.Errorf("roundtrip failed for bool")
		}
	})

	t.Run("JSONSlice roundtrip", func(t *testing.T) {
		original := JSONSlice[string]{"a", "b", "c"}

		value, err := original.Value()
		if err != nil {
			t.Fatalf("Value() error = %v", err)
		}

		var restored JSONSlice[string]
		err = restored.Scan(value)
		if err != nil {
			t.Fatalf("Scan() error = %v", err)
		}

		if len(restored) != 3 || restored[0] != "a" {
			t.Errorf("roundtrip failed: got %v", restored)
		}
	})
}
