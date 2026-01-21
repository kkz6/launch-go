package ptr

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeref(t *testing.T) {
	t.Run("returns value when non-nil", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", Deref(&s))
	})

	t.Run("returns zero value when nil", func(t *testing.T) {
		var s *string
		assert.Equal(t, "", Deref(s))
	})

	t.Run("works with int", func(t *testing.T) {
		i := 42
		assert.Equal(t, 42, Deref(&i))

		var nilInt *int
		assert.Equal(t, 0, Deref(nilInt))
	})

	t.Run("works with bool", func(t *testing.T) {
		b := true
		assert.Equal(t, true, Deref(&b))

		var nilBool *bool
		assert.Equal(t, false, Deref(nilBool))
	})
}

func TestDerefOr(t *testing.T) {
	t.Run("returns value when non-nil", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", DerefOr(&s, "default"))
	})

	t.Run("returns default when nil", func(t *testing.T) {
		var s *string
		assert.Equal(t, "default", DerefOr(s, "default"))
	})

	t.Run("works with int", func(t *testing.T) {
		var nilInt *int
		assert.Equal(t, 42, DerefOr(nilInt, 42))
	})
}

func TestPtr(t *testing.T) {
	t.Run("creates pointer to string", func(t *testing.T) {
		p := Ptr("hello")
		assert.NotNil(t, p)
		assert.Equal(t, "hello", *p)
	})

	t.Run("creates pointer to int", func(t *testing.T) {
		p := Ptr(42)
		assert.NotNil(t, p)
		assert.Equal(t, 42, *p)
	})

	t.Run("creates pointer to struct", func(t *testing.T) {
		type Person struct {
			Name string
		}
		p := Ptr(Person{Name: "John"})
		assert.NotNil(t, p)
		assert.Equal(t, "John", p.Name)
	})
}

func TestNilIfZero(t *testing.T) {
	t.Run("returns nil for zero int", func(t *testing.T) {
		assert.Nil(t, NilIfZero(0))
	})

	t.Run("returns pointer for non-zero int", func(t *testing.T) {
		p := NilIfZero(42)
		assert.NotNil(t, p)
		assert.Equal(t, 42, *p)
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		assert.Nil(t, NilIfZero(""))
	})

	t.Run("returns pointer for non-empty string", func(t *testing.T) {
		p := NilIfZero("hello")
		assert.NotNil(t, p)
		assert.Equal(t, "hello", *p)
	})
}

func TestNilIfEmpty(t *testing.T) {
	t.Run("returns nil for empty string", func(t *testing.T) {
		assert.Nil(t, NilIfEmpty(""))
	})

	t.Run("returns pointer for non-empty string", func(t *testing.T) {
		p := NilIfEmpty("hello")
		assert.NotNil(t, p)
		assert.Equal(t, "hello", *p)
	})
}

func TestEmptyIfNil(t *testing.T) {
	t.Run("returns empty for nil", func(t *testing.T) {
		assert.Equal(t, "", EmptyIfNil(nil))
	})

	t.Run("returns value for non-nil", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", EmptyIfNil(&s))
	})
}

func TestEqual(t *testing.T) {
	t.Run("both nil returns true", func(t *testing.T) {
		var a, b *string
		assert.True(t, Equal(a, b))
	})

	t.Run("one nil returns false", func(t *testing.T) {
		s := "hello"
		assert.False(t, Equal(&s, nil))
		assert.False(t, Equal(nil, &s))
	})

	t.Run("equal values returns true", func(t *testing.T) {
		a, b := "hello", "hello"
		assert.True(t, Equal(&a, &b))
	})

	t.Run("different values returns false", func(t *testing.T) {
		a, b := "hello", "world"
		assert.False(t, Equal(&a, &b))
	})
}

func TestCoalesce(t *testing.T) {
	t.Run("returns first non-nil value", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", Coalesce(nil, &s, Ptr("world")))
	})

	t.Run("returns zero when all nil", func(t *testing.T) {
		var a, b, c *string
		assert.Equal(t, "", Coalesce(a, b, c))
	})

	t.Run("works with single non-nil", func(t *testing.T) {
		s := "only"
		assert.Equal(t, "only", Coalesce(&s))
	})
}

func TestCoalescePtr(t *testing.T) {
	t.Run("returns first non-nil pointer", func(t *testing.T) {
		s := "hello"
		result := CoalescePtr(nil, &s, Ptr("world"))
		assert.NotNil(t, result)
		assert.Equal(t, "hello", *result)
	})

	t.Run("returns nil when all nil", func(t *testing.T) {
		var a, b, c *string
		assert.Nil(t, CoalescePtr(a, b, c))
	})
}

func TestMap(t *testing.T) {
	t.Run("applies function when non-nil", func(t *testing.T) {
		s := "hello"
		result := Map(&s, strings.ToUpper)
		assert.NotNil(t, result)
		assert.Equal(t, "HELLO", *result)
	})

	t.Run("returns nil when nil", func(t *testing.T) {
		var s *string
		result := Map(s, strings.ToUpper)
		assert.Nil(t, result)
	})

	t.Run("works with type conversion", func(t *testing.T) {
		s := "hello"
		result := Map(&s, func(v string) int { return len(v) })
		assert.NotNil(t, result)
		assert.Equal(t, 5, *result)
	})
}

func TestMapOr(t *testing.T) {
	t.Run("applies function when non-nil", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, 5, MapOr(&s, func(v string) int { return len(v) }, 0))
	})

	t.Run("returns default when nil", func(t *testing.T) {
		var s *string
		assert.Equal(t, -1, MapOr(s, func(v string) int { return len(v) }, -1))
	})
}
