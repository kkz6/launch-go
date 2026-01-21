package health

import (
	"context"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents the result of a single health check
type Check struct {
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Message string        `json:"message,omitempty"`
	Latency time.Duration `json:"latency_ms"`
}

// Result represents the aggregated health check result
type Result struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Checks    []Check   `json:"checks"`
}

// Checker is the interface that health checkers must implement
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Aggregator manages multiple health checkers and runs them concurrently
type Aggregator struct {
	checkers []Checker
	mu       sync.RWMutex
}

// NewAggregator creates a new health check aggregator
func NewAggregator() *Aggregator {
	return &Aggregator{
		checkers: make([]Checker, 0),
	}
}

// Add adds a checker to the aggregator and returns the aggregator for chaining
func (a *Aggregator) Add(c Checker) *Aggregator {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.checkers = append(a.checkers, c)

	return a
}

// Check runs all registered health checks concurrently and returns the aggregated result
func (a *Aggregator) Check(ctx context.Context) Result {
	a.mu.RLock()
	checkers := make([]Checker, len(a.checkers))
	copy(checkers, a.checkers)
	a.mu.RUnlock()

	result := Result{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC(),
		Checks:    make([]Check, len(checkers)),
	}

	if len(checkers) == 0 {
		return result
	}

	var wg sync.WaitGroup
	wg.Add(len(checkers))

	for i, checker := range checkers {
		go func(idx int, c Checker) {
			defer wg.Done()

			check := Check{
				Name:   c.Name(),
				Status: StatusHealthy,
			}

			start := time.Now()
			err := c.Check(ctx)
			check.Latency = time.Since(start)

			if err != nil {
				check.Status = StatusUnhealthy
				check.Message = err.Error()
			}

			result.Checks[idx] = check
		}(i, checker)
	}

	wg.Wait()

	for _, check := range result.Checks {
		if check.Status == StatusUnhealthy {
			result.Status = StatusDegraded
			break
		}
	}

	return result
}
