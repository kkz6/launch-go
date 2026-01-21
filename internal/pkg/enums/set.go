package enums

// Set provides common enum operations like validation and terminal state checking
type Set[T comparable] struct {
	values   []T
	terminal map[T]bool
}

// NewSet creates a new enum set with the given values and terminal states
func NewSet[T comparable](values []T, terminal ...T) *Set[T] {
	s := &Set[T]{
		values:   values,
		terminal: make(map[T]bool),
	}
	for _, t := range terminal {
		s.terminal[t] = true
	}
	return s
}

// Values returns all valid values
func (s *Set[T]) Values() []T { return s.values }

// IsValid checks if a value is in the set
func (s *Set[T]) IsValid(v T) bool {
	for _, val := range s.values {
		if v == val {
			return true
		}
	}
	return false
}

// IsTerminal checks if a value is a terminal state
func (s *Set[T]) IsTerminal(v T) bool { return s.terminal[v] }
