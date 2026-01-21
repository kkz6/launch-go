package security

import "testing"

func TestSecureCompare(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "equal strings",
			a:    "secret_token_123",
			b:    "secret_token_123",
			want: true,
		},
		{
			name: "different strings",
			a:    "secret_token_123",
			b:    "secret_token_456",
			want: false,
		},
		{
			name: "different lengths",
			a:    "short",
			b:    "much_longer_string",
			want: false,
		},
		{
			name: "empty strings",
			a:    "",
			b:    "",
			want: true,
		},
		{
			name: "one empty string",
			a:    "secret",
			b:    "",
			want: false,
		},
		{
			name: "unicode strings equal",
			a:    "hello\u0000world",
			b:    "hello\u0000world",
			want: true,
		},
		{
			name: "unicode strings different",
			a:    "hello\u0000world",
			b:    "hello\u0001world",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SecureCompare(tt.a, tt.b); got != tt.want {
				t.Errorf("SecureCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecureCompareBytes(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{
			name: "equal bytes",
			a:    []byte{0x01, 0x02, 0x03},
			b:    []byte{0x01, 0x02, 0x03},
			want: true,
		},
		{
			name: "different bytes",
			a:    []byte{0x01, 0x02, 0x03},
			b:    []byte{0x01, 0x02, 0x04},
			want: false,
		},
		{
			name: "different lengths",
			a:    []byte{0x01, 0x02},
			b:    []byte{0x01, 0x02, 0x03},
			want: false,
		},
		{
			name: "empty slices",
			a:    []byte{},
			b:    []byte{},
			want: true,
		},
		{
			name: "nil slices",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "one nil slice",
			a:    []byte{0x01},
			b:    nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SecureCompareBytes(tt.a, tt.b); got != tt.want {
				t.Errorf("SecureCompareBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

// BenchmarkSecureCompare ensures constant-time behavior
// The benchmark should show similar times regardless of where strings differ
func BenchmarkSecureCompare_EqualStrings(b *testing.B) {
	a := "this_is_a_secret_webhook_token_12345"
	b2 := "this_is_a_secret_webhook_token_12345"
	for i := 0; i < b.N; i++ {
		SecureCompare(a, b2)
	}
}

func BenchmarkSecureCompare_DifferentFirstChar(b *testing.B) {
	a := "this_is_a_secret_webhook_token_12345"
	b2 := "xhis_is_a_secret_webhook_token_12345"
	for i := 0; i < b.N; i++ {
		SecureCompare(a, b2)
	}
}

func BenchmarkSecureCompare_DifferentLastChar(b *testing.B) {
	a := "this_is_a_secret_webhook_token_12345"
	b2 := "this_is_a_secret_webhook_token_12346"
	for i := 0; i < b.N; i++ {
		SecureCompare(a, b2)
	}
}
