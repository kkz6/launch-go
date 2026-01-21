package xutil

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeGet(t *testing.T) {
	t.Run("key exists", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		val, ok := SafeGet(m, "a")
		assert.True(t, ok)
		assert.Equal(t, 1, val)
	})

	t.Run("key does not exist", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		val, ok := SafeGet(m, "c")
		assert.False(t, ok)
		assert.Equal(t, 0, val)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		val, ok := SafeGet(m, "a")
		assert.False(t, ok)
		assert.Equal(t, 0, val)
	})
}

func TestSafeGetOr(t *testing.T) {
	t.Run("key exists", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		val := SafeGetOr(m, "a", 99)
		assert.Equal(t, 1, val)
	})

	t.Run("key does not exist", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		val := SafeGetOr(m, "c", 99)
		assert.Equal(t, 99, val)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		val := SafeGetOr(m, "a", 99)
		assert.Equal(t, 99, val)
	})
}

func TestKeys(t *testing.T) {
	t.Run("returns all keys", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2, "c": 3}
		keys := Keys(m)
		sort.Strings(keys)
		assert.Equal(t, []string{"a", "b", "c"}, keys)
	})

	t.Run("empty map", func(t *testing.T) {
		m := map[string]int{}
		keys := Keys(m)
		assert.Empty(t, keys)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		keys := Keys(m)
		assert.Nil(t, keys)
	})
}

func TestValues(t *testing.T) {
	t.Run("returns all values", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2, "c": 3}
		values := Values(m)
		sort.Ints(values)
		assert.Equal(t, []int{1, 2, 3}, values)
	})

	t.Run("empty map", func(t *testing.T) {
		m := map[string]int{}
		values := Values(m)
		assert.Empty(t, values)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		values := Values(m)
		assert.Nil(t, values)
	})
}

func TestMerge(t *testing.T) {
	t.Run("merges multiple maps", func(t *testing.T) {
		m1 := map[string]int{"a": 1, "b": 2}
		m2 := map[string]int{"c": 3, "d": 4}
		merged := Merge(m1, m2)
		assert.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}, merged)
	})

	t.Run("later map overwrites", func(t *testing.T) {
		m1 := map[string]int{"a": 1, "b": 2}
		m2 := map[string]int{"b": 3, "c": 4}
		merged := Merge(m1, m2)
		assert.Equal(t, 3, merged["b"])
	})

	t.Run("empty maps", func(t *testing.T) {
		merged := Merge[string, int]()
		assert.Empty(t, merged)
	})
}

func TestMapKeys(t *testing.T) {
	t.Run("transforms keys", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		result := MapKeys(m, strings.ToUpper)
		assert.Equal(t, map[string]int{"A": 1, "B": 2}, result)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		result := MapKeys(m, strings.ToUpper)
		assert.Nil(t, result)
	})
}

func TestMapValues(t *testing.T) {
	t.Run("transforms values", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		result := MapValues(m, func(v int) int { return v * 2 })
		assert.Equal(t, map[string]int{"a": 2, "b": 4}, result)
	})

	t.Run("type conversion", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		result := MapValues(m, func(v int) string {
			if v == 1 {
				return "one"
			}
			return "two"
		})
		assert.Equal(t, map[string]string{"a": "one", "b": "two"}, result)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		result := MapValues(m, func(v int) int { return v * 2 })
		assert.Nil(t, result)
	})
}

func TestFilterMap(t *testing.T) {
	t.Run("filters by predicate", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}
		result := FilterMap(m, func(k string, v int) bool { return v%2 == 0 })
		assert.Equal(t, map[string]int{"b": 2, "d": 4}, result)
	})

	t.Run("filter by key", func(t *testing.T) {
		m := map[string]int{"abc": 1, "ab": 2, "a": 3}
		result := FilterMap(m, func(k string, v int) bool { return len(k) >= 2 })
		assert.Equal(t, map[string]int{"abc": 1, "ab": 2}, result)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		result := FilterMap(m, func(k string, v int) bool { return true })
		assert.Nil(t, result)
	})
}

func TestHasKey(t *testing.T) {
	t.Run("key exists", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		assert.True(t, HasKey(m, "a"))
	})

	t.Run("key does not exist", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		assert.False(t, HasKey(m, "c"))
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		assert.False(t, HasKey(m, "a"))
	})
}

func TestInvert(t *testing.T) {
	t.Run("inverts map", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		result := Invert(m)
		assert.Equal(t, map[int]string{1: "a", 2: "b"}, result)
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		result := Invert(m)
		assert.Nil(t, result)
	})
}
