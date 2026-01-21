package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// testEnum is a mock enum type for testing
type testEnum string

const (
	testEnumActive   testEnum = "active"
	testEnumInactive testEnum = "inactive"
	testEnumPending  testEnum = "pending"
)

func (e testEnum) String() string {
	return string(e)
}

func (e testEnum) Label() string {
	labels := map[testEnum]string{
		testEnumActive:   "Active",
		testEnumInactive: "Inactive",
		testEnumPending:  "Pending",
	}
	if label, ok := labels[e]; ok {
		return label
	}
	return string(e)
}

// testEnumWithDesc is a mock enum with description for testing
type testEnumWithDesc string

const (
	testEnumDescOne testEnumWithDesc = "one"
	testEnumDescTwo testEnumWithDesc = "two"
)

func (e testEnumWithDesc) String() string {
	return string(e)
}

func (e testEnumWithDesc) Label() string {
	labels := map[testEnumWithDesc]string{
		testEnumDescOne: "One",
		testEnumDescTwo: "Two",
	}
	if label, ok := labels[e]; ok {
		return label
	}
	return string(e)
}

func (e testEnumWithDesc) Description() string {
	desc := map[testEnumWithDesc]string{
		testEnumDescOne: "First option",
		testEnumDescTwo: "Second option",
	}
	if d, ok := desc[e]; ok {
		return d
	}
	return ""
}

func TestEnumToResponse(t *testing.T) {
	t.Run("converts enum to response", func(t *testing.T) {
		resp := EnumToResponse(testEnumActive)
		assert.Equal(t, "active", resp.Value)
		assert.Equal(t, "Active", resp.Label)
	})

	t.Run("handles different enum values", func(t *testing.T) {
		resp := EnumToResponse(testEnumPending)
		assert.Equal(t, "pending", resp.Value)
		assert.Equal(t, "Pending", resp.Label)
	})
}

func TestEnumsToResponses(t *testing.T) {
	t.Run("converts slice of enums", func(t *testing.T) {
		enums := []testEnum{testEnumActive, testEnumInactive}
		responses := EnumsToResponses(enums)

		assert.Len(t, responses, 2)
		assert.Equal(t, "active", responses[0].Value)
		assert.Equal(t, "Active", responses[0].Label)
		assert.Equal(t, "inactive", responses[1].Value)
		assert.Equal(t, "Inactive", responses[1].Label)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		var enums []testEnum
		responses := EnumsToResponses(enums)
		assert.Nil(t, responses)
	})

	t.Run("returns empty slice for empty input", func(t *testing.T) {
		enums := []testEnum{}
		responses := EnumsToResponses(enums)
		assert.NotNil(t, responses)
		assert.Len(t, responses, 0)
	})
}

func TestEnumToResponseWithDesc(t *testing.T) {
	t.Run("includes description", func(t *testing.T) {
		resp := EnumToResponseWithDesc(testEnumDescOne)
		assert.Equal(t, "one", resp.Value)
		assert.Equal(t, "One", resp.Label)
		assert.NotNil(t, resp.Description)
		assert.Equal(t, "First option", *resp.Description)
	})
}

func TestNewEnumResponse(t *testing.T) {
	t.Run("creates from raw strings", func(t *testing.T) {
		resp := NewEnumResponse("active", "Active Status")
		assert.Equal(t, "active", resp.Value)
		assert.Equal(t, "Active Status", resp.Label)
	})
}

func TestEnumMapToResponses(t *testing.T) {
	t.Run("converts map to responses", func(t *testing.T) {
		m := map[string]string{
			"a": "Alpha",
			"b": "Beta",
		}
		responses := EnumMapToResponses(m)
		assert.Len(t, responses, 2)

		// Since map iteration order is not guaranteed, check both exist
		found := make(map[string]bool)
		for _, r := range responses {
			found[r.Value] = true
			if r.Value == "a" {
				assert.Equal(t, "Alpha", r.Label)
			} else if r.Value == "b" {
				assert.Equal(t, "Beta", r.Label)
			}
		}
		assert.True(t, found["a"])
		assert.True(t, found["b"])
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		responses := EnumMapToResponses(nil)
		assert.Nil(t, responses)
	})
}
