package testutil

import (
	"sync"

	"gorm.io/gorm"
)

// MockStore is a generic in-memory store for testing.
// It provides common CRUD operations with error injection support,
// reducing boilerplate in test mock repositories.
//
// Example usage:
//
//	type MockServerRepo struct {
//	    servers *MockStore[models.Server]
//	}
//
//	func NewMockServerRepo() *MockServerRepo {
//	    return &MockServerRepo{
//	        servers: NewMockStore[models.Server](),
//	    }
//	}
//
//	func (r *MockServerRepo) Create(ctx context.Context, s *models.Server) error {
//	    return r.servers.Create(s, func(s *models.Server) string { return s.ID })
//	}
type MockStore[T any] struct {
	mu     sync.RWMutex
	items  map[string]*T
	errors map[string]error
}

// NewMockStore creates a new generic mock store.
func NewMockStore[T any]() *MockStore[T] {
	return &MockStore[T]{
		items:  make(map[string]*T),
		errors: make(map[string]error),
	}
}

// SetError configures an error to be returned for a specific method.
// Method names should match the operation: "Create", "FindByID", "Update", "Delete", "FindAll".
func (m *MockStore[T]) SetError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[method] = err
}

// ClearError removes a configured error for a specific method.
func (m *MockStore[T]) ClearError(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.errors, method)
}

// ClearAllErrors removes all configured errors.
func (m *MockStore[T]) ClearAllErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = make(map[string]error)
}

// getError returns a configured error if one exists.
func (m *MockStore[T]) getError(method string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errors[method]
}

// Create adds an item to the store.
// The getID function extracts the unique identifier from the item.
func (m *MockStore[T]) Create(item *T, getID func(*T) string) error {
	if err := m.getError("Create"); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[getID(item)] = item
	return nil
}

// FindByID retrieves an item by its ID.
// Returns gorm.ErrRecordNotFound if the item does not exist.
func (m *MockStore[T]) FindByID(id string) (*T, error) {
	if err := m.getError("FindByID"); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return item, nil
}

// FindByIDOrNil retrieves an item by its ID.
// Returns nil (without error) if the item does not exist.
func (m *MockStore[T]) FindByIDOrNil(id string) (*T, error) {
	if err := m.getError("FindByIDOrNil"); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.items[id], nil
}

// Update replaces an item in the store.
// The getID function extracts the unique identifier from the item.
func (m *MockStore[T]) Update(item *T, getID func(*T) string) error {
	if err := m.getError("Update"); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[getID(item)] = item
	return nil
}

// Delete removes an item from the store by ID.
func (m *MockStore[T]) Delete(id string) error {
	if err := m.getError("Delete"); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, id)
	return nil
}

// FindAll returns all items in the store.
func (m *MockStore[T]) FindAll() ([]*T, error) {
	if err := m.getError("FindAll"); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*T, 0, len(m.items))
	for _, item := range m.items {
		result = append(result, item)
	}
	return result, nil
}

// FindWhere returns items matching a predicate function.
func (m *MockStore[T]) FindWhere(predicate func(*T) bool) ([]*T, error) {
	if err := m.getError("FindWhere"); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*T, 0)
	for _, item := range m.items {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result, nil
}

// FindOneWhere returns the first item matching a predicate function.
// Returns gorm.ErrRecordNotFound if no item matches.
func (m *MockStore[T]) FindOneWhere(predicate func(*T) bool) (*T, error) {
	if err := m.getError("FindOneWhere"); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, item := range m.items {
		if predicate(item) {
			return item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// Exists checks if an item with the given ID exists.
func (m *MockStore[T]) Exists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.items[id]
	return ok
}

// Count returns the number of items in the store.
func (m *MockStore[T]) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// CountWhere returns the number of items matching a predicate.
func (m *MockStore[T]) CountWhere(predicate func(*T) bool) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, item := range m.items {
		if predicate(item) {
			count++
		}
	}
	return count
}

// Clear removes all items from the store.
func (m *MockStore[T]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]*T)
}

// Reset clears all items and errors from the store.
func (m *MockStore[T]) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]*T)
	m.errors = make(map[string]error)
}

// Seed populates the store with items.
// The getID function extracts the unique identifier from each item.
func (m *MockStore[T]) Seed(items []*T, getID func(*T) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		m.items[getID(item)] = item
	}
}

// GetAll returns a copy of all items (for test assertions).
func (m *MockStore[T]) GetAll() map[string]*T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*T, len(m.items))
	for k, v := range m.items {
		result[k] = v
	}
	return result
}

// IDExtractor is a helper type for common ID extraction patterns.
type IDExtractor[T any] func(*T) string

// WithStringID creates an ID extractor for items with a string ID field.
// Usage: store.Create(item, WithStringID[MyModel](func(m *MyModel) string { return m.ID }))
func WithStringID[T any](fn func(*T) string) IDExtractor[T] {
	return fn
}
