package table

import "sync"

// Registry is the lookup used by the action endpoint to map an incoming
// table name to the Table that owns the action handlers. It's safe for
// concurrent reads after initial registration.
type Registry struct {
	mu     sync.RWMutex
	tables map[string]Table
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{tables: map[string]Table{}}
}

// Register adds a table to the registry, keyed on its Config.Name.
func (r *Registry) Register(t Table) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tables[t.Config().Name] = t
}

// Get returns a registered table (nil if unknown).
func (r *Registry) Get(name string) Table {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tables[name]
}
