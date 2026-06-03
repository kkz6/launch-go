package table

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToFloat(t *testing.T) {
	cases := []struct {
		in   any
		want float64
		ok   bool
	}{
		{float64(3.5), 3.5, true},
		{float32(2), 2, true},
		{int(7), 7, true},
		{int64(9), 9, true},
		{"3.14", 3.14, true},
		{"-0.5", -0.5, true},
		{"not-a-number", 0, false},
		{true, 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := toFloat(c.in)
		assert.Equalf(t, c.ok, ok, "toFloat(%v) ok", c.in)
		if c.ok {
			assert.InDeltaf(t, c.want, got, 0.0001, "toFloat(%v)", c.in)
		}
	}
}

func TestParseDateISO(t *testing.T) {
	cases := []struct {
		in   any
		want string
		ok   bool
	}{
		{"2024-03-15", "2024-03-15", true},
		{"2024-03-15T10:30:00Z", "2024-03-15", true},
		{time.Date(2024, 3, 15, 23, 0, 0, 0, time.UTC), "2024-03-15", true},
		{"15/03/2024", "", false},
		{"garbage", "", false},
		{42, "", false},
	}
	for _, c := range cases {
		got, ok := parseDateISO(c.in)
		assert.Equalf(t, c.ok, ok, "parseDateISO(%v) ok", c.in)
		if c.ok {
			assert.Equalf(t, c.want, got, "parseDateISO(%v)", c.in)
		}
	}
}

func TestChoosePaginationType(t *testing.T) {
	assert.Equal(t, PaginationFull, choosePaginationType(""), "empty defaults to full")
	assert.Equal(t, PaginationCursor, choosePaginationType(PaginationCursor))
	assert.Equal(t, PaginationSimple, choosePaginationType(PaginationSimple))
}
