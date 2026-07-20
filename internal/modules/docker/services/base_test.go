package services

import "testing"

func TestMapResponseValues(t *testing.T) {
	t.Run("preserves a nil slice", func(t *testing.T) {
		if got := mapResponseValues[struct{}, string](nil, func(*struct{}) *string { return nil }); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("converts each row in order", func(t *testing.T) {
		rows := []int{1, 2, 3}
		got := mapResponseValues(rows, func(value *int) *string {
			response := string(rune('0' + *value))
			return &response
		})

		want := []string{"1", "2", "3"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("response %d = %q, want %q", i, got[i], want[i])
			}
		}
	})
}
