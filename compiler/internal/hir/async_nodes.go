package hir

// M8A — Async/future surface nodes in HIR.
// These are "plumbing" ops only; lowering/state machine expansion happens later.

// FutureNew allocates/creates a future handle.
// Example print:   %t1 = future.new
type FutureNew struct {
	Dst Temp
}

func (*FutureNew) isStmt() {}

// FutureComplete completes a future with a value.
// Example print:   future.complete %fut, %val
type FutureComplete struct {
	Fut Value
	Val Value
}

func (*FutureComplete) isStmt() {}

// Await awaits a future and produces a value into a temp.
// In sync contexts, codegen maps this to a blocking await stub.
// ResultType (e.g., "i32", "ptr") tells the LLVM backend how to narrow
// the i64 transport value returned by __await_blocking.
// Example print:   await %fut -> %t2
type Await struct {
	Fut        Value
	Dst        Temp
	ResultType string // LLVM type to narrow to: "i32", "ptr", "i1", "double", ""
}

func (*Await) isStmt() {}

// FutureSpawn spawns a body function on a background thread with args.
// The BodyFn is called with (future_ptr, args...) and its return value
// is used to complete the future automatically.
// Example print:   future.spawn %fut, @body_fn, [arg0, arg1]
type FutureSpawn struct {
	Fut    Value   // future handle (ptr)
	BodyFn string  // name of the body function
	Args   []Value // user arguments to pass
}

func (*FutureSpawn) isStmt() {}
