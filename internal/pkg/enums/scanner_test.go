package enums

import (
	"database/sql/driver"
	"errors"
	"testing"
)

// ScannerTestEnum is a test enum type for scanner tests
type ScannerTestEnum string

const (
	ScannerTestEnumPending ScannerTestEnum = "pending"
	ScannerTestEnumActive  ScannerTestEnum = "active"
	ScannerTestEnumFailed  ScannerTestEnum = "failed"
)

func TestScan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected ScannerTestEnum
		wantErr  bool
	}{
		{
			name:     "scan string value",
			input:    "pending",
			expected: ScannerTestEnumPending,
			wantErr:  false,
		},
		{
			name:     "scan byte slice value",
			input:    []byte("active"),
			expected: ScannerTestEnumActive,
			wantErr:  false,
		},
		{
			name:     "scan nil value",
			input:    nil,
			expected: ScannerTestEnum(""),
			wantErr:  false,
		},
		{
			name:     "scan empty string",
			input:    "",
			expected: ScannerTestEnum(""),
			wantErr:  false,
		},
		{
			name:     "scan empty byte slice",
			input:    []byte(""),
			expected: ScannerTestEnum(""),
			wantErr:  false,
		},
		{
			name:     "scan invalid type int",
			input:    123,
			expected: ScannerTestEnum(""),
			wantErr:  true,
		},
		{
			name:     "scan invalid type float",
			input:    12.34,
			expected: ScannerTestEnum(""),
			wantErr:  true,
		},
		{
			name:     "scan invalid type bool",
			input:    true,
			expected: ScannerTestEnum(""),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result ScannerTestEnum
			err := Scan(&result, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}

				if !errors.Is(err, ErrInvalidEnumScan) {
					t.Errorf("expected ErrInvalidEnumScan, got %v", err)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestValue(t *testing.T) {
	tests := []struct {
		name     string
		input    ScannerTestEnum
		expected driver.Value
		wantErr  bool
	}{
		{
			name:     "value from enum",
			input:    ScannerTestEnumPending,
			expected: "pending",
			wantErr:  false,
		},
		{
			name:     "value from active enum",
			input:    ScannerTestEnumActive,
			expected: "active",
			wantErr:  false,
		},
		{
			name:     "value from empty enum",
			input:    ScannerTestEnum(""),
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Value(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		input    ScannerTestEnum
		expected string
	}{
		{
			name:     "string from pending enum",
			input:    ScannerTestEnumPending,
			expected: "pending",
		},
		{
			name:     "string from active enum",
			input:    ScannerTestEnumActive,
			expected: "active",
		},
		{
			name:     "string from empty enum",
			input:    ScannerTestEnum(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := String(tt.input)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestScanNullable(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected ScannerTestEnum
		wantNull bool
		wantErr  bool
	}{
		{
			name:     "scan string value",
			input:    "pending",
			expected: ScannerTestEnumPending,
			wantNull: false,
			wantErr:  false,
		},
		{
			name:     "scan byte slice value",
			input:    []byte("active"),
			expected: ScannerTestEnumActive,
			wantNull: false,
			wantErr:  false,
		},
		{
			name:     "scan nil value returns isNull true",
			input:    nil,
			expected: ScannerTestEnum(""),
			wantNull: true,
			wantErr:  false,
		},
		{
			name:     "scan empty string is not null",
			input:    "",
			expected: ScannerTestEnum(""),
			wantNull: false,
			wantErr:  false,
		},
		{
			name:     "scan invalid type returns error",
			input:    123,
			expected: ScannerTestEnum(""),
			wantNull: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result ScannerTestEnum
			isNull, err := ScanNullable(&result, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if isNull != tt.wantNull {
				t.Errorf("expected isNull=%v, got %v", tt.wantNull, isNull)
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestValueNullable(t *testing.T) {
	tests := []struct {
		name     string
		input    ScannerTestEnum
		expected driver.Value
		wantErr  bool
	}{
		{
			name:     "value from enum",
			input:    ScannerTestEnumPending,
			expected: "pending",
			wantErr:  false,
		},
		{
			name:     "empty enum returns nil",
			input:    ScannerTestEnum(""),
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValueNullable(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    ScannerTestEnum
		expected bool
	}{
		{
			name:     "empty enum is empty",
			input:    ScannerTestEnum(""),
			expected: true,
		},
		{
			name:     "non-empty enum is not empty",
			input:    ScannerTestEnumPending,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmpty(tt.input)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEquals(t *testing.T) {
	tests := []struct {
		name     string
		a        ScannerTestEnum
		b        ScannerTestEnum
		expected bool
	}{
		{
			name:     "equal enums",
			a:        ScannerTestEnumPending,
			b:        ScannerTestEnumPending,
			expected: true,
		},
		{
			name:     "different enums",
			a:        ScannerTestEnumPending,
			b:        ScannerTestEnumActive,
			expected: false,
		},
		{
			name:     "empty enums are equal",
			a:        ScannerTestEnum(""),
			b:        ScannerTestEnum(""),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equals(tt.a, tt.b)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIn(t *testing.T) {
	tests := []struct {
		name     string
		value    ScannerTestEnum
		values   []ScannerTestEnum
		expected bool
	}{
		{
			name:     "value in list",
			value:    ScannerTestEnumPending,
			values:   []ScannerTestEnum{ScannerTestEnumPending, ScannerTestEnumActive},
			expected: true,
		},
		{
			name:     "value not in list",
			value:    ScannerTestEnumFailed,
			values:   []ScannerTestEnum{ScannerTestEnumPending, ScannerTestEnumActive},
			expected: false,
		},
		{
			name:     "empty list",
			value:    ScannerTestEnumPending,
			values:   []ScannerTestEnum{},
			expected: false,
		},
		{
			name:     "single value match",
			value:    ScannerTestEnumPending,
			values:   []ScannerTestEnum{ScannerTestEnumPending},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := In(tt.value, tt.values...)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNotIn(t *testing.T) {
	tests := []struct {
		name     string
		value    ScannerTestEnum
		values   []ScannerTestEnum
		expected bool
	}{
		{
			name:     "value in list returns false",
			value:    ScannerTestEnumPending,
			values:   []ScannerTestEnum{ScannerTestEnumPending, ScannerTestEnumActive},
			expected: false,
		},
		{
			name:     "value not in list returns true",
			value:    ScannerTestEnumFailed,
			values:   []ScannerTestEnum{ScannerTestEnumPending, ScannerTestEnumActive},
			expected: true,
		},
		{
			name:     "empty list returns true",
			value:    ScannerTestEnumPending,
			values:   []ScannerTestEnum{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NotIn(tt.value, tt.values...)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestScanPreservesPointerSemantics ensures that Scan modifies the pointed-to value
func TestScanPreservesPointerSemantics(t *testing.T) {
	var e = ScannerTestEnumPending

	err := Scan(&e, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e != ScannerTestEnumActive {
		t.Errorf("expected value to be modified to 'active', got %q", e)
	}
}

// TestValueDoesNotModifyOriginal ensures that Value doesn't modify the original
func TestValueDoesNotModifyOriginal(t *testing.T) {
	e := ScannerTestEnumPending

	_, err := Value(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e != ScannerTestEnumPending {
		t.Errorf("expected value to remain 'pending', got %q", e)
	}
}

// TestRoundTrip ensures that Value and Scan work together correctly
func TestRoundTrip(t *testing.T) {
	original := ScannerTestEnumActive

	dbValue, err := Value(original)
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}

	var restored ScannerTestEnum
	err = Scan(&restored, dbValue)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	if original != restored {
		t.Errorf("round trip failed: original=%q, restored=%q", original, restored)
	}
}

// TestRoundTripNullable ensures that ValueNullable and ScanNullable work together
func TestRoundTripNullable(t *testing.T) {
	t.Run("non-null value", func(t *testing.T) {
		original := ScannerTestEnumActive

		dbValue, err := ValueNullable(original)
		if err != nil {
			t.Fatalf("ValueNullable error: %v", err)
		}

		var restored ScannerTestEnum
		isNull, err := ScanNullable(&restored, dbValue)
		if err != nil {
			t.Fatalf("ScanNullable error: %v", err)
		}

		if isNull {
			t.Error("expected isNull to be false")
		}

		if original != restored {
			t.Errorf("round trip failed: original=%q, restored=%q", original, restored)
		}
	})

	t.Run("null value", func(t *testing.T) {
		original := ScannerTestEnum("")

		dbValue, err := ValueNullable(original)
		if err != nil {
			t.Fatalf("ValueNullable error: %v", err)
		}

		if dbValue != nil {
			t.Errorf("expected nil for empty enum, got %v", dbValue)
		}

		var restored ScannerTestEnum
		isNull, err := ScanNullable(&restored, dbValue)
		if err != nil {
			t.Fatalf("ScanNullable error: %v", err)
		}

		if !isNull {
			t.Error("expected isNull to be true")
		}

		if restored != "" {
			t.Errorf("expected empty string, got %q", restored)
		}
	})
}
