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
// In async contexts, this node will not survive final lowering (expanded by M8B).
// Example print:   await %fut -> %t2
type Await struct {
	Fut Value
	Dst Temp
}

func (*Await) isStmt() {}
