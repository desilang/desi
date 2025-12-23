package types

// TaskGroup represents a structured concurrency scope.
// All spawned tasks must complete before the group exits.
// Provides automatic cleanup and cancellation propagation.
type TaskGroup struct {
	// TaskGroup doesn't hold a type parameter - it manages heterogeneous tasks
}

func (*TaskGroup) isType() {}
func (t *TaskGroup) String() string {
	return "TaskGroup"
}

// TaskGroupOf returns a singleton TaskGroup type
func TaskGroupOf() *TaskGroup {
	return &TaskGroup{}
}
