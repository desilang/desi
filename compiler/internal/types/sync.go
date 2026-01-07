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

// RwLock represents a reader-writer lock protecting a value of type Inner.
// Multiple readers can hold the lock concurrently, but writers are exclusive.
type RwLock struct {
	Inner T // The type being protected
}

func (*RwLock) isType() {}
func (rw *RwLock) String() string {
	return "RwLock[" + rw.Inner.String() + "]"
}

// ReadGuard represents an acquired read lock.
// Multiple ReadGuards can exist simultaneously.
type ReadGuard struct {
	Inner T // The type being protected (read-only access)
}

func (*ReadGuard) isType() {}
func (g *ReadGuard) String() string {
	return "ReadGuard[" + g.Inner.String() + "]"
}

// WriteGuard represents an acquired write lock.
// Only one WriteGuard can exist at a time.
type WriteGuard struct {
	Inner T // The type being protected (read-write access)
}

func (*WriteGuard) isType() {}
func (g *WriteGuard) String() string {
	return "WriteGuard[" + g.Inner.String() + "]"
}

// RwLockOf creates a RwLock type wrapping the given type
func RwLockOf(inner T) *RwLock {
	return &RwLock{Inner: inner}
}

// ReadGuardOf creates a ReadGuard type wrapping the given type
func ReadGuardOf(inner T) *ReadGuard {
	return &ReadGuard{Inner: inner}
}

// WriteGuardOf creates a WriteGuard type wrapping the given type
func WriteGuardOf(inner T) *WriteGuard {
	return &WriteGuard{Inner: inner}
}
