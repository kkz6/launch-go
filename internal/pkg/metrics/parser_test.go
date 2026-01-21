package metrics

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestParser_ParseFloat(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		want      float64
		wantError bool
	}{
		{
			name:      "valid float",
			value:     "123.45",
			fieldName: "test_field",
			want:      123.45,
			wantError: false,
		},
		{
			name:      "valid integer as float",
			value:     "100",
			fieldName: "test_field",
			want:      100.0,
			wantError: false,
		},
		{
			name:      "empty string returns zero",
			value:     "",
			fieldName: "test_field",
			want:      0,
			wantError: false,
		},
		{
			name:      "invalid string logs error",
			value:     "not-a-number",
			fieldName: "test_field",
			want:      0,
			wantError: true,
		},
		{
			name:      "negative value",
			value:     "-50.5",
			fieldName: "test_field",
			want:      -50.5,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			p := NewParser(logger, "test-server-id")

			got := p.ParseFloat(tt.value, tt.fieldName)

			if got != tt.want {
				t.Errorf("ParseFloat() = %v, want %v", got, tt.want)
			}

			if tt.wantError && !p.HasErrors() {
				t.Error("Expected error to be recorded")
			}

			if !tt.wantError && p.HasErrors() {
				t.Errorf("Unexpected error recorded: %v", p.Errors())
			}
		})
	}
}

func TestParser_ParseInt(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldName string
		want      int64
		wantError bool
	}{
		{
			name:      "valid integer",
			value:     "12345",
			fieldName: "test_field",
			want:      12345,
			wantError: false,
		},
		{
			name:      "empty string returns zero",
			value:     "",
			fieldName: "test_field",
			want:      0,
			wantError: false,
		},
		{
			name:      "invalid string logs error",
			value:     "not-a-number",
			fieldName: "test_field",
			want:      0,
			wantError: true,
		},
		{
			name:      "float string logs error",
			value:     "123.45",
			fieldName: "test_field",
			want:      0,
			wantError: true,
		},
		{
			name:      "negative value",
			value:     "-500",
			fieldName: "test_field",
			want:      -500,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			p := NewParser(logger, "test-server-id")

			got := p.ParseInt(tt.value, tt.fieldName)

			if got != tt.want {
				t.Errorf("ParseInt() = %v, want %v", got, tt.want)
			}

			if tt.wantError && !p.HasErrors() {
				t.Error("Expected error to be recorded")
			}

			if !tt.wantError && p.HasErrors() {
				t.Errorf("Unexpected error recorded: %v", p.Errors())
			}
		})
	}
}

func TestParser_ErrorTracking(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	p := NewParser(logger, "test-server-id")

	// Initially no errors
	if p.HasErrors() {
		t.Error("Expected no initial errors")
	}
	if p.ErrorCount() != 0 {
		t.Errorf("Expected 0 errors, got %d", p.ErrorCount())
	}

	// Parse invalid values
	p.ParseFloat("invalid1", "field1")
	p.ParseFloat("invalid2", "field2")
	p.ParseInt("invalid3", "field3")

	// Should have 3 errors
	if !p.HasErrors() {
		t.Error("Expected errors to be recorded")
	}
	if p.ErrorCount() != 3 {
		t.Errorf("Expected 3 errors, got %d", p.ErrorCount())
	}

	errors := p.Errors()
	expected := []string{"field1", "field2", "field3"}
	for i, e := range expected {
		if errors[i] != e {
			t.Errorf("Expected error[%d] = %s, got %s", i, e, errors[i])
		}
	}
}

func TestParser_LogsContainContext(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	serverID := "srv-12345"
	p := NewParser(logger, serverID)

	p.ParseFloat("invalid", "disk_total")

	logOutput := buf.String()

	// Check that log contains server_id
	if !bytes.Contains(buf.Bytes(), []byte("srv-12345")) {
		t.Errorf("Log output should contain server_id, got: %s", logOutput)
	}

	// Check that log contains field name
	if !bytes.Contains(buf.Bytes(), []byte("disk_total")) {
		t.Errorf("Log output should contain field name, got: %s", logOutput)
	}

	// Check that log contains the invalid value
	if !bytes.Contains(buf.Bytes(), []byte("invalid")) {
		t.Errorf("Log output should contain the invalid value, got: %s", logOutput)
	}
}
