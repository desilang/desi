package types

// Mutex represents a mutex protecting a value of type Inner.
// Usage: Mutex.new(value) or Mutex[T].new(value)
type Mutex struct {
	Inner T // The type being protected
}

func (*Mutex) isType() {}
func (m *Mutex) String() string {
	return "Mutex[" + m.Inner.String() + "]"
}

// MutexGuard represents an acquired mutex lock.
// While the guard exists, the mutex is held.
// When the guard goes out of scope (RAII), the lock is released.
type MutexGuard struct {
	Inner T // The type being protected (for accessing the value)
}

func (*MutexGuard) isType() {}
func (g *MutexGuard) String() string {
	return "MutexGuard[" + g.Inner.String() + "]"
}

// MutexOf creates a Mutex type wrapping the given type
func MutexOf(inner T) *Mutex {
	return &Mutex{Inner: inner}
}

// MutexGuardOf creates a MutexGuard type wrapping the given type
func MutexGuardOf(inner T) *MutexGuard {
	return &MutexGuard{Inner: inner}
}
