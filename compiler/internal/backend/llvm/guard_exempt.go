package llvm

import "github.com/desilang/desi/compiler/internal/hir"

// GuardExemptFunctions returns the functions that provably cannot recurse, and
// therefore need no stack guard.
//
// The rule is deliberately the narrowest one that is sound without any further
// analysis: a function that makes no calls at all cannot recurse. To be entered
// a second time while already on the stack, something has to run and call it —
// and a function that calls nothing runs nothing. That holds however it was
// reached, including through a function pointer from the runtime, which is what
// makes it checkable without tracking where function references travel.
//
// About a third of the functions in the example suite qualify: getters, small
// arithmetic helpers, predicates — exactly the shape that gets called in a hot
// loop, where the guard is worth the most per call.
//
// The wider rule — a call graph, its strongly connected components, and a guard
// only for functions inside a cycle — needs to know every place a function's
// address is taken, since an address-taken function can be re-entered from the
// runtime along an edge the direct call graph does not have. That means a
// complete walk of every value in every HIR statement, and a node type missed
// there is not a wrong answer that shows up in a test: it is a function wrongly
// declared safe, which faults instead of raising, in a program that recurses in
// a way nothing here exercised. Worth doing with a real walker; not worth
// approximating.
func GuardExemptFunctions(funcs []*hir.Func) map[string]bool {
	exempt := make(map[string]bool, len(funcs))
	for _, fn := range funcs {
		if fn == nil || fn.Name == "" {
			continue
		}
		if !makesAnyCall(fn) {
			exempt[fn.Name] = true
		}
	}
	return exempt
}

// makesAnyCall reports whether the function contains a call of any kind —
// to a user function, to the runtime, or in tail position.
//
// Calls are statements in the HIR, never nested inside a value, so this sees
// all of them by walking the statement lists. That is what keeps the check
// complete without a general-purpose walker.
func makesAnyCall(fn *hir.Func) bool {
	for _, b := range fn.Blocks {
		if b == nil {
			continue
		}
		for _, st := range b.Stmts {
			switch st.(type) {
			case *hir.Call, *hir.TailCall:
				return true
			}
		}
	}
	return false
}

// SetGuardExempt records the functions that need no stack guard.
func (m *Module) SetGuardExempt(names map[string]bool) {
	m.guardExempt = names
}
