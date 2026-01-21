package syncutil

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestCached_GetOrCompute(t *testing.T) {
	var c Cached[int]
	callCount := 0

	// First call should compute
	result := c.GetOrCompute(func() int {
		callCount++
		return 42
	})

	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if callCount != 1 {
		t.Errorf("expected compute to be called once, was called %d times", callCount)
	}

	// Second call should return cached value
	result = c.GetOrCompute(func() int {
		callCount++
		return 100
	})

	if result != 42 {
		t.Errorf("expected cached value 42, got %d", result)
	}
	if callCount != 1 {
		t.Errorf("expected compute to still be called once, was called %d times", callCount)
	}
}

func TestCached_Get(t *testing.T) {
	var c Cached[string]

	// Should return zero value and false when not set
	val, ok := c.Get()
	if ok {
		t.Error("expected ok to be false for unset value")
	}
	if val != "" {
		t.Errorf("expected zero value, got %q", val)
	}

	// Set a value
	c.Set("hello")

	// Should return the value and true
	val, ok = c.Get()
	if !ok {
		t.Error("expected ok to be true for set value")
	}
	if val != "hello" {
		t.Errorf("expected 'hello', got %q", val)
	}
}

func TestCached_Invalidate(t *testing.T) {
	var c Cached[int]
	callCount := 0

	// Set initial value
	c.GetOrCompute(func() int {
		callCount++
		return 1
	})

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}

	// Invalidate
	c.Invalidate()

	// Should recompute
	result := c.GetOrCompute(func() int {
		callCount++
		return 2
	})

	if result != 2 {
		t.Errorf("expected 2, got %d", result)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestCached_Set(t *testing.T) {
	var c Cached[string]

	c.Set("test")

	result := c.GetOrCompute(func() string {
		return "should not be called"
	})

	if result != "test" {
		t.Errorf("expected 'test', got %q", result)
	}
}

func TestCached_IsSet(t *testing.T) {
	var c Cached[int]

	if c.IsSet() {
		t.Error("expected IsSet to be false initially")
	}

	c.Set(1)

	if !c.IsSet() {
		t.Error("expected IsSet to be true after Set")
	}

	c.Invalidate()

	if c.IsSet() {
		t.Error("expected IsSet to be false after Invalidate")
	}
}

func TestCached_Concurrent(t *testing.T) {
	var c Cached[int]
	var computeCount int32

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.GetOrCompute(func() int {
				atomic.AddInt32(&computeCount, 1)
				return 42
			})
		}()
	}

	wg.Wait()

	// The compute function should only be called once
	if computeCount != 1 {
		t.Errorf("expected compute to be called once, was called %d times", computeCount)
	}

	result, ok := c.Get()
	if !ok || result != 42 {
		t.Errorf("expected cached value 42, got %d (ok=%v)", result, ok)
	}
}

func TestCached_PointerType(t *testing.T) {
	type Data struct {
		Value string
	}

	var c Cached[*Data]

	result := c.GetOrCompute(func() *Data {
		return &Data{Value: "test"}
	})

	if result == nil || result.Value != "test" {
		t.Errorf("expected Data with Value 'test', got %v", result)
	}

	// Verify caching works with pointer types
	result2 := c.GetOrCompute(func() *Data {
		return &Data{Value: "should not see this"}
	})

	if result != result2 {
		t.Error("expected same pointer to be returned")
	}
}

func TestCached_ZeroValue(t *testing.T) {
	// Test that zero values are properly cached
	var c Cached[int]

	result := c.GetOrCompute(func() int {
		return 0
	})

	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}

	// The value should be cached even though it's zero
	if !c.IsSet() {
		t.Error("expected IsSet to be true even for zero value")
	}
}
