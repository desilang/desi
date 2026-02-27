package types

// Supervisor represents a thread pool with auto-restart.
// Provides persistent children + work queue pool.
type Supervisor struct {
	// Supervisor doesn't hold a type parameter - manages generic work items
}

func (*Supervisor) isType() {}
func (t *Supervisor) String() string {
	return "Supervisor"
}

// SupervisorOf returns a singleton Supervisor type
func SupervisorOf() *Supervisor {
	return &Supervisor{}
}
