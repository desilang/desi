package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// Which loops may rewind the arena at their latch.
//
// runEscapeAnalysis answers one question: does this value outlive the
// *function*? Releasing a loop body's allocations at the end of every iteration
// asks a strictly finer one: does it outlive the *iteration*? A value can be
// arena-safe by the first measure and unsafe by the second —
//
//	let mut keep = []
//	for i in range(10):
//	    let t = [i]
//	    keep.append(t)
//
// Neither `keep` nor `t` escapes the function, so both are arena candidates
// today. Rewinding at the latch would leave `keep` holding ten pointers into
// bytes that have been handed back out.
//
// There is a second way to get the same crash, and it does not involve storing
// anything:
//
//	let mut acc = []
//	for i in range(10):
//	    acc.append(i)
//
// `acc`'s backing array lives in the arena, and growing it calls __arena_alloc
// again — from inside the loop. A rewind would release the array the list is
// still using.
//
// One rule covers both: a loop may rewind only if **no arena-allocated value
// declared outside it is mentioned anywhere inside it**. In the first case that
// excludes the loop because `keep` is mentioned; in the second because `acc`
// is. Every other route by which an inner value could outlive its iteration —
// returning it, storing it in a field, putting it in a global, handing it to a
// function that might retain it — already makes it escape the *function*, which
// takes it out of the arena entirely and off this analysis's books.
//
// The rule is deliberately blunter than it has to be. Merely reading an outer
// collection cannot grow it, so `len(acc)` inside the loop is harmless and is
// nonetheless treated as disqualifying. Being wrong in that direction costs a
// loop its rewind. Being wrong in the other direction costs a use-after-free.

// findRewindableLoops records the loops in this function whose body allocations
// are all dead by the end of each iteration.
func (v *escapeVisitor) findRewindableLoops(body *ast.Block, info *Info) {
	if body == nil {
		return
	}

	// Every loop in the function, innermost and outermost alike. Nested loops
	// are judged independently: an inner loop can qualify while the loop around
	// it does not, and the arena's marks nest to match.
	var loops []ast.Node
	inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.WhileStmt:
			loops = append(loops, n)
		}
		return true
	})
	if len(loops) == 0 {
		return
	}

	// The values that actually live in the arena: candidates that survived
	// escape propagation.
	arena := make(map[*Symbol]bool, len(v.candidates))
	for sym := range v.candidates {
		if v.isArena(sym) {
			arena[sym] = true
		}
	}
	if len(arena) == 0 {
		return
	}

	for _, loop := range loops {
		declared := v.symbolsDeclaredIn(loop)

		// Something has to be reclaimable for the rewind to be worth emitting.
		inner := 0
		for sym := range arena {
			if declared[sym] {
				inner++
			}
		}
		if inner == 0 {
			continue
		}

		// Nothing declared inside the loop may flow to anything declared
		// outside it. The dependency edges the escape constraints already built
		// say exactly that: deps[dst] lists what flows into dst, so an edge
		// whose source is inside the loop and whose destination is not is a
		// value outliving the iteration that produced it.
		//
		// The test is on the edge itself, not on whether an arena value crosses
		// it, and that is deliberate. Tracking arena-ness across the boundary
		// means following chains — an arena list stored in a heap list stored
		// in an outer variable is still reachable after the rewind — and a
		// missed link there is a use-after-free rather than a missed
		// optimisation. Any crossing blocks.
		blocked := false
		for dst, srcs := range v.deps {
			if declared[dst] {
				continue // stays inside the loop
			}
			for _, src := range srcs {
				if declared[src] {
					blocked = true
					break
				}
			}
			if blocked {
				break
			}
		}
		if blocked {
			continue
		}
		if info.RewindableLoops == nil {
			info.RewindableLoops = map[ast.Node]bool{}
		}
		info.RewindableLoops[loop] = true
	}
}

// symbolsDeclaredIn returns the symbols bound by a `let` anywhere inside the
// node. For nested loops this includes the inner loop's declarations, which is
// what the outer loop wants: they are allocated and finished with during one of
// its iterations.
func (v *escapeVisitor) symbolsDeclaredIn(n ast.Node) map[*Symbol]bool {
	out := map[*Symbol]bool{}
	inspect(n, func(x ast.Node) bool {
		let, ok := x.(*ast.LetStmt)
		if !ok {
			return true
		}
		if let.Name.Name != "" {
			if sym := v.info.Idents[&let.Name]; sym != nil {
				out[sym] = true
			}
		}
		for i := range let.Pattern {
			if sym := v.info.Idents[&let.Pattern[i]]; sym != nil {
				out[sym] = true
			}
		}
		return true
	})
	return out
}
