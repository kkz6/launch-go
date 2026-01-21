package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	t.Run("removes duplicates", func(t *testing.T) {
		result := Unique([]int{1, 2, 2, 3, 1, 4})
		assert.Equal(t, []int{1, 2, 3, 4}, result)
	})

	t.Run("preserves order of first occurrence", func(t *testing.T) {
		result := Unique([]string{"c", "a", "b", "a", "c"})
		assert.Equal(t, []string{"c", "a", "b"}, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := Unique([]int{})
		assert.Empty(t, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []int
		result := Unique(s)
		assert.Nil(t, result)
	})

	t.Run("no duplicates", func(t *testing.T) {
		result := Unique([]int{1, 2, 3})
		assert.Equal(t, []int{1, 2, 3}, result)
	})
}

func TestFirst(t *testing.T) {
	t.Run("returns first element", func(t *testing.T) {
		val, ok := First([]int{1, 2, 3})
		assert.True(t, ok)
		assert.Equal(t, 1, val)
	})

	t.Run("empty slice", func(t *testing.T) {
		val, ok := First([]int{})
		assert.False(t, ok)
		assert.Equal(t, 0, val)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []string
		val, ok := First(s)
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})
}

func TestFirstOr(t *testing.T) {
	t.Run("returns first element", func(t *testing.T) {
		val := FirstOr([]int{1, 2, 3}, 99)
		assert.Equal(t, 1, val)
	})

	t.Run("returns default for empty", func(t *testing.T) {
		val := FirstOr([]int{}, 99)
		assert.Equal(t, 99, val)
	})
}

func TestLast(t *testing.T) {
	t.Run("returns last element", func(t *testing.T) {
		val, ok := Last([]int{1, 2, 3})
		assert.True(t, ok)
		assert.Equal(t, 3, val)
	})

	t.Run("empty slice", func(t *testing.T) {
		val, ok := Last([]int{})
		assert.False(t, ok)
		assert.Equal(t, 0, val)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []string
		val, ok := Last(s)
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})
}

func TestLastOr(t *testing.T) {
	t.Run("returns last element", func(t *testing.T) {
		val := LastOr([]int{1, 2, 3}, 99)
		assert.Equal(t, 3, val)
	})

	t.Run("returns default for empty", func(t *testing.T) {
		val := LastOr([]int{}, 99)
		assert.Equal(t, 99, val)
	})
}

func TestFilter(t *testing.T) {
	t.Run("filters elements", func(t *testing.T) {
		result := Filter([]int{1, 2, 3, 4, 5}, func(n int) bool { return n%2 == 0 })
		assert.Equal(t, []int{2, 4}, result)
	})

	t.Run("no matches", func(t *testing.T) {
		result := Filter([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 })
		assert.Empty(t, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []int
		result := Filter(s, func(n int) bool { return true })
		assert.Nil(t, result)
	})
}

func TestMap(t *testing.T) {
	t.Run("transforms elements", func(t *testing.T) {
		result := Map([]int{1, 2, 3}, func(n int) int { return n * 2 })
		assert.Equal(t, []int{2, 4, 6}, result)
	})

	t.Run("type conversion", func(t *testing.T) {
		result := Map([]int{1, 2, 3}, func(n int) string {
			return string(rune('a' + n - 1))
		})
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []int
		result := Map(s, func(n int) int { return n * 2 })
		assert.Nil(t, result)
	})
}

func TestReduce(t *testing.T) {
	t.Run("sums elements", func(t *testing.T) {
		result := Reduce([]int{1, 2, 3, 4}, 0, func(acc, n int) int { return acc + n })
		assert.Equal(t, 10, result)
	})

	t.Run("concatenates strings", func(t *testing.T) {
		result := Reduce([]string{"a", "b", "c"}, "", func(acc, s string) string { return acc + s })
		assert.Equal(t, "abc", result)
	})

	t.Run("empty slice returns initial", func(t *testing.T) {
		result := Reduce([]int{}, 42, func(acc, n int) int { return acc + n })
		assert.Equal(t, 42, result)
	})
}

func TestFind(t *testing.T) {
	t.Run("finds element", func(t *testing.T) {
		val, ok := Find([]int{1, 2, 3, 4}, func(n int) bool { return n > 2 })
		assert.True(t, ok)
		assert.Equal(t, 3, val)
	})

	t.Run("no match", func(t *testing.T) {
		val, ok := Find([]int{1, 2, 3}, func(n int) bool { return n > 10 })
		assert.False(t, ok)
		assert.Equal(t, 0, val)
	})
}

func TestFindIndex(t *testing.T) {
	t.Run("finds index", func(t *testing.T) {
		idx := FindIndex([]int{1, 2, 3, 4}, func(n int) bool { return n > 2 })
		assert.Equal(t, 2, idx)
	})

	t.Run("no match returns -1", func(t *testing.T) {
		idx := FindIndex([]int{1, 2, 3}, func(n int) bool { return n > 10 })
		assert.Equal(t, -1, idx)
	})
}

func TestAny(t *testing.T) {
	t.Run("returns true when any match", func(t *testing.T) {
		result := Any([]int{1, 2, 3}, func(n int) bool { return n > 2 })
		assert.True(t, result)
	})

	t.Run("returns false when none match", func(t *testing.T) {
		result := Any([]int{1, 2, 3}, func(n int) bool { return n > 10 })
		assert.False(t, result)
	})

	t.Run("returns false for empty slice", func(t *testing.T) {
		result := Any([]int{}, func(n int) bool { return true })
		assert.False(t, result)
	})
}

func TestAll(t *testing.T) {
	t.Run("returns true when all match", func(t *testing.T) {
		result := All([]int{2, 4, 6}, func(n int) bool { return n%2 == 0 })
		assert.True(t, result)
	})

	t.Run("returns false when any fails", func(t *testing.T) {
		result := All([]int{2, 3, 6}, func(n int) bool { return n%2 == 0 })
		assert.False(t, result)
	})

	t.Run("returns true for empty slice", func(t *testing.T) {
		result := All([]int{}, func(n int) bool { return false })
		assert.True(t, result)
	})
}

func TestNone(t *testing.T) {
	t.Run("returns true when none match", func(t *testing.T) {
		result := None([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 })
		assert.True(t, result)
	})

	t.Run("returns false when any match", func(t *testing.T) {
		result := None([]int{1, 2, 3}, func(n int) bool { return n%2 == 0 })
		assert.False(t, result)
	})

	t.Run("returns true for empty slice", func(t *testing.T) {
		result := None([]int{}, func(n int) bool { return true })
		assert.True(t, result)
	})
}

func TestChunk(t *testing.T) {
	t.Run("splits into chunks", func(t *testing.T) {
		result := Chunk([]int{1, 2, 3, 4, 5}, 2)
		assert.Equal(t, [][]int{{1, 2}, {3, 4}, {5}}, result)
	})

	t.Run("exact chunks", func(t *testing.T) {
		result := Chunk([]int{1, 2, 3, 4}, 2)
		assert.Equal(t, [][]int{{1, 2}, {3, 4}}, result)
	})

	t.Run("size larger than slice", func(t *testing.T) {
		result := Chunk([]int{1, 2}, 5)
		assert.Equal(t, [][]int{{1, 2}}, result)
	})

	t.Run("size of 1", func(t *testing.T) {
		result := Chunk([]int{1, 2, 3}, 1)
		assert.Equal(t, [][]int{{1}, {2}, {3}}, result)
	})

	t.Run("zero size returns nil", func(t *testing.T) {
		result := Chunk([]int{1, 2, 3}, 0)
		assert.Nil(t, result)
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		result := Chunk([]int{}, 2)
		assert.Nil(t, result)
	})
}

func TestFlatten(t *testing.T) {
	t.Run("flattens nested slices", func(t *testing.T) {
		result := Flatten([][]int{{1, 2}, {3, 4}, {5}})
		assert.Equal(t, []int{1, 2, 3, 4, 5}, result)
	})

	t.Run("handles empty inner slices", func(t *testing.T) {
		result := Flatten([][]int{{1, 2}, {}, {3}})
		assert.Equal(t, []int{1, 2, 3}, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s [][]int
		result := Flatten(s)
		assert.Nil(t, result)
	})
}

func TestReverse(t *testing.T) {
	t.Run("reverses slice", func(t *testing.T) {
		result := Reverse([]int{1, 2, 3, 4, 5})
		assert.Equal(t, []int{5, 4, 3, 2, 1}, result)
	})

	t.Run("single element", func(t *testing.T) {
		result := Reverse([]int{1})
		assert.Equal(t, []int{1}, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := Reverse([]int{})
		assert.Empty(t, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []int
		result := Reverse(s)
		assert.Nil(t, result)
	})
}

func TestCompact(t *testing.T) {
	t.Run("removes zero values", func(t *testing.T) {
		result := Compact([]string{"a", "", "b", "", "c"})
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("removes zero ints", func(t *testing.T) {
		result := Compact([]int{1, 0, 2, 0, 3})
		assert.Equal(t, []int{1, 2, 3}, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []string
		result := Compact(s)
		assert.Nil(t, result)
	})
}

func TestIndex(t *testing.T) {
	t.Run("finds element", func(t *testing.T) {
		idx := Index([]string{"a", "b", "c"}, "b")
		assert.Equal(t, 1, idx)
	})

	t.Run("not found returns -1", func(t *testing.T) {
		idx := Index([]string{"a", "b", "c"}, "d")
		assert.Equal(t, -1, idx)
	})
}

func TestContainsAny(t *testing.T) {
	t.Run("contains one of elements", func(t *testing.T) {
		assert.True(t, ContainsAny([]string{"a", "b", "c"}, "b", "d"))
	})

	t.Run("contains none of elements", func(t *testing.T) {
		assert.False(t, ContainsAny([]string{"a", "b", "c"}, "d", "e"))
	})

	t.Run("empty elements", func(t *testing.T) {
		assert.False(t, ContainsAny([]string{"a", "b", "c"}))
	})
}

func TestContainsAll(t *testing.T) {
	t.Run("contains all elements", func(t *testing.T) {
		assert.True(t, ContainsAll([]string{"a", "b", "c"}, "a", "b"))
	})

	t.Run("missing one element", func(t *testing.T) {
		assert.False(t, ContainsAll([]string{"a", "b", "c"}, "a", "d"))
	})

	t.Run("empty elements returns true", func(t *testing.T) {
		assert.True(t, ContainsAll([]string{"a", "b", "c"}))
	})
}
