package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testModel struct {
	ID     string
	Name   string
	Status string
	Value  int
}

type testResponse struct {
	ID   string
	Name string
}

func toResponse(m *testModel) testResponse {
	return testResponse{ID: m.ID, Name: m.Name}
}

func toResponsePtr(m *testModel) *testResponse {
	return &testResponse{ID: m.ID, Name: m.Name}
}

func TestConvertSlice(t *testing.T) {
	t.Run("converts slice of models", func(t *testing.T) {
		models := []testModel{
			{ID: "1", Name: "Alice"},
			{ID: "2", Name: "Bob"},
		}
		results := ConvertSlice(models, toResponse)
		assert.Len(t, results, 2)
		assert.Equal(t, "1", results[0].ID)
		assert.Equal(t, "Alice", results[0].Name)
		assert.Equal(t, "2", results[1].ID)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		results := ConvertSlice(models, toResponse)
		assert.Nil(t, results)
	})

	t.Run("returns empty slice for empty input", func(t *testing.T) {
		models := []testModel{}
		results := ConvertSlice(models, toResponse)
		assert.NotNil(t, results)
		assert.Len(t, results, 0)
	})
}

func TestTransformSlice(t *testing.T) {
	t.Run("transforms slice using provided function", func(t *testing.T) {
		models := []testModel{
			{ID: "1", Name: "Alice"},
			{ID: "2", Name: "Bob"},
			{ID: "3", Name: "Charlie"},
		}
		results := TransformSlice(models, toResponse)
		assert.Len(t, results, 3)
		assert.Equal(t, "1", results[0].ID)
		assert.Equal(t, "Alice", results[0].Name)
		assert.Equal(t, "2", results[1].ID)
		assert.Equal(t, "Bob", results[1].Name)
		assert.Equal(t, "3", results[2].ID)
		assert.Equal(t, "Charlie", results[2].Name)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		results := TransformSlice(models, toResponse)
		assert.Nil(t, results)
	})

	t.Run("returns empty slice for empty input", func(t *testing.T) {
		models := []testModel{}
		results := TransformSlice(models, toResponse)
		assert.NotNil(t, results)
		assert.Len(t, results, 0)
	})

	t.Run("works with inline transform functions", func(t *testing.T) {
		models := []testModel{{ID: "1"}, {ID: "2"}}
		ids := TransformSlice(models, func(m *testModel) string { return m.ID })
		assert.Equal(t, []string{"1", "2"}, ids)
	})
}

func TestConvertSlicePtr(t *testing.T) {
	t.Run("converts to pointer slice", func(t *testing.T) {
		models := []testModel{{ID: "1", Name: "Alice"}}
		results := ConvertSlicePtr(models, toResponsePtr)
		assert.Len(t, results, 1)
		assert.NotNil(t, results[0])
		assert.Equal(t, "1", results[0].ID)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		results := ConvertSlicePtr(models, toResponsePtr)
		assert.Nil(t, results)
	})
}

func TestConvertPtr(t *testing.T) {
	t.Run("converts non-nil pointer", func(t *testing.T) {
		m := &testModel{ID: "1", Name: "Alice"}
		result := ConvertPtr(m, toResponsePtr)
		assert.NotNil(t, result)
		assert.Equal(t, "1", result.ID)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var m *testModel
		result := ConvertPtr(m, toResponsePtr)
		assert.Nil(t, result)
	})
}

func TestConvertPtrValue(t *testing.T) {
	t.Run("converts non-nil pointer to value", func(t *testing.T) {
		m := &testModel{ID: "1", Name: "Alice"}
		result := ConvertPtrValue(m, toResponse)
		assert.Equal(t, "1", result.ID)
	})

	t.Run("returns zero value for nil input", func(t *testing.T) {
		var m *testModel
		result := ConvertPtrValue(m, toResponse)
		assert.Equal(t, "", result.ID)
		assert.Equal(t, "", result.Name)
	})
}

func TestMapSlice(t *testing.T) {
	t.Run("maps to different type", func(t *testing.T) {
		models := []testModel{{ID: "1"}, {ID: "2"}}
		ids := MapSlice(models, func(m *testModel) string { return m.ID })
		assert.Equal(t, []string{"1", "2"}, ids)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		result := MapSlice(models, func(m *testModel) string { return m.ID })
		assert.Nil(t, result)
	})
}

func TestMapSliceValue(t *testing.T) {
	t.Run("maps values", func(t *testing.T) {
		models := []testModel{{ID: "1"}, {ID: "2"}}
		ids := MapSliceValue(models, func(m testModel) string { return m.ID })
		assert.Equal(t, []string{"1", "2"}, ids)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		result := MapSliceValue(models, func(m testModel) string { return m.ID })
		assert.Nil(t, result)
	})
}

func TestFilterSlice(t *testing.T) {
	t.Run("filters based on predicate", func(t *testing.T) {
		models := []testModel{
			{ID: "1", Status: "active"},
			{ID: "2", Status: "inactive"},
			{ID: "3", Status: "active"},
		}
		active := FilterSlice(models, func(m *testModel) bool { return m.Status == "active" })
		assert.Len(t, active, 2)
		assert.Equal(t, "1", active[0].ID)
		assert.Equal(t, "3", active[1].ID)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		result := FilterSlice(models, func(m *testModel) bool { return true })
		assert.Nil(t, result)
	})

	t.Run("returns empty for no matches", func(t *testing.T) {
		models := []testModel{{ID: "1", Status: "inactive"}}
		result := FilterSlice(models, func(m *testModel) bool { return m.Status == "active" })
		assert.Empty(t, result)
	})
}

func TestFirstOrNil(t *testing.T) {
	t.Run("returns first element", func(t *testing.T) {
		slice := []testModel{{ID: "1"}, {ID: "2"}}
		result := FirstOrNil(slice)
		assert.NotNil(t, result)
		assert.Equal(t, "1", result.ID)
	})

	t.Run("returns nil for empty slice", func(t *testing.T) {
		slice := []testModel{}
		result := FirstOrNil(slice)
		assert.Nil(t, result)
	})

	t.Run("returns nil for nil slice", func(t *testing.T) {
		var slice []testModel
		result := FirstOrNil(slice)
		assert.Nil(t, result)
	})
}

func TestReduce(t *testing.T) {
	t.Run("sums values", func(t *testing.T) {
		models := []testModel{{Value: 10}, {Value: 20}, {Value: 30}}
		total := Reduce(models, 0, func(acc int, m *testModel) int { return acc + m.Value })
		assert.Equal(t, 60, total)
	})

	t.Run("concatenates strings", func(t *testing.T) {
		models := []testModel{{Name: "a"}, {Name: "b"}, {Name: "c"}}
		result := Reduce(models, "", func(acc string, m *testModel) string { return acc + m.Name })
		assert.Equal(t, "abc", result)
	})

	t.Run("returns initial for empty slice", func(t *testing.T) {
		var models []testModel
		result := Reduce(models, 100, func(acc int, m *testModel) int { return acc + m.Value })
		assert.Equal(t, 100, result)
	})
}

func TestReduceValue(t *testing.T) {
	t.Run("sums integers", func(t *testing.T) {
		nums := []int{1, 2, 3, 4, 5}
		total := ReduceValue(nums, 0, func(acc, n int) int { return acc + n })
		assert.Equal(t, 15, total)
	})
}

func TestGroupBy(t *testing.T) {
	t.Run("groups by status", func(t *testing.T) {
		models := []testModel{
			{ID: "1", Status: "active"},
			{ID: "2", Status: "inactive"},
			{ID: "3", Status: "active"},
		}
		groups := GroupBy(models, func(m *testModel) string { return m.Status })
		assert.Len(t, groups, 2)
		assert.Len(t, groups["active"], 2)
		assert.Len(t, groups["inactive"], 1)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		result := GroupBy(models, func(m *testModel) string { return m.Status })
		assert.Nil(t, result)
	})
}

func TestUnique(t *testing.T) {
	t.Run("removes duplicates by ID", func(t *testing.T) {
		models := []testModel{
			{ID: "1", Name: "First"},
			{ID: "2", Name: "Second"},
			{ID: "1", Name: "Duplicate"},
		}
		result := Unique(models, func(m *testModel) string { return m.ID })
		assert.Len(t, result, 2)
		assert.Equal(t, "First", result[0].Name)
		assert.Equal(t, "Second", result[1].Name)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		result := Unique(models, func(m *testModel) string { return m.ID })
		assert.Nil(t, result)
	})
}

func TestContains(t *testing.T) {
	t.Run("returns true when found", func(t *testing.T) {
		models := []testModel{{ID: "1"}, {ID: "2"}}
		assert.True(t, Contains(models, func(m *testModel) bool { return m.ID == "2" }))
	})

	t.Run("returns false when not found", func(t *testing.T) {
		models := []testModel{{ID: "1"}, {ID: "2"}}
		assert.False(t, Contains(models, func(m *testModel) bool { return m.ID == "3" }))
	})

	t.Run("returns false for empty slice", func(t *testing.T) {
		var models []testModel
		assert.False(t, Contains(models, func(m *testModel) bool { return true }))
	})
}

func TestFind(t *testing.T) {
	t.Run("returns first match", func(t *testing.T) {
		models := []testModel{{ID: "1", Name: "First"}, {ID: "2", Name: "Second"}}
		result := Find(models, func(m *testModel) bool { return m.ID == "2" })
		assert.NotNil(t, result)
		assert.Equal(t, "Second", result.Name)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		models := []testModel{{ID: "1"}}
		result := Find(models, func(m *testModel) bool { return m.ID == "999" })
		assert.Nil(t, result)
	})
}

func TestIndexOf(t *testing.T) {
	t.Run("returns index when found", func(t *testing.T) {
		models := []testModel{{ID: "a"}, {ID: "b"}, {ID: "c"}}
		assert.Equal(t, 1, IndexOf(models, func(m *testModel) bool { return m.ID == "b" }))
	})

	t.Run("returns -1 when not found", func(t *testing.T) {
		models := []testModel{{ID: "a"}, {ID: "b"}}
		assert.Equal(t, -1, IndexOf(models, func(m *testModel) bool { return m.ID == "z" }))
	})
}

func TestChunk(t *testing.T) {
	t.Run("splits into chunks", func(t *testing.T) {
		models := []testModel{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}}
		chunks := Chunk(models, 2)
		assert.Len(t, chunks, 3)
		assert.Len(t, chunks[0], 2)
		assert.Len(t, chunks[1], 2)
		assert.Len(t, chunks[2], 1)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		result := Chunk(models, 2)
		assert.Nil(t, result)
	})

	t.Run("returns nil for zero size", func(t *testing.T) {
		models := []testModel{{ID: "1"}}
		result := Chunk(models, 0)
		assert.Nil(t, result)
	})

	t.Run("handles size larger than slice", func(t *testing.T) {
		models := []testModel{{ID: "1"}, {ID: "2"}}
		chunks := Chunk(models, 10)
		assert.Len(t, chunks, 1)
		assert.Len(t, chunks[0], 2)
	})
}

func TestPartition(t *testing.T) {
	t.Run("splits by predicate", func(t *testing.T) {
		models := []testModel{
			{ID: "1", Status: "active"},
			{ID: "2", Status: "inactive"},
			{ID: "3", Status: "active"},
		}
		active, inactive := Partition(models, func(m *testModel) bool { return m.Status == "active" })
		assert.Len(t, active, 2)
		assert.Len(t, inactive, 1)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var models []testModel
		matching, nonMatching := Partition(models, func(m *testModel) bool { return true })
		assert.Nil(t, matching)
		assert.Nil(t, nonMatching)
	})
}

func TestSafeDeref(t *testing.T) {
	t.Run("returns value for non-nil", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", SafeDeref(&s))
	})

	t.Run("returns zero for nil", func(t *testing.T) {
		var s *string
		assert.Equal(t, "", SafeDeref(s))
	})
}

func TestPtr(t *testing.T) {
	t.Run("creates pointer to string", func(t *testing.T) {
		p := Ptr("hello")
		assert.NotNil(t, p)
		assert.Equal(t, "hello", *p)
	})
}
