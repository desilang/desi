package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parse"
)

// rewindableCount checks src and reports how many loops were cleared to rewind
// the arena at their latch.
func rewindableCount(t *testing.T, src string) int {
	t.Helper()
	mod, pdiags := parse.ParseFile("<mem>", []byte(strings.TrimLeft(src, "\n")))
	if len(pdiags) != 0 {
		t.Fatalf("parse diags: %+v", pdiags)
	}
	diags, info := Check(mod)
	if len(diags) != 0 {
		t.Fatalf("check diags: %+v", diags)
	}
	if info == nil {
		t.Fatal("no check info")
	}
	n := 0
	for loop, ok := range info.RewindableLoops {
		if ok {
			if _, isFor := loop.(*ast.ForStmt); !isFor {
				if _, isWhile := loop.(*ast.WhileStmt); !isWhile {
					t.Fatalf("a non-loop node was marked rewindable: %T", loop)
				}
			}
			n++
		}
	}
	return n
}

// The shape the whole exercise is for: allocate, use, discard, repeat.
func TestRewindLoopLocalAllocation(t *testing.T) {
	if got := rewindableCount(t, `
def churn() -> int:
	let mut i = 0
	let mut n = 0
	while i < 100:
		let t = [i, i, i]
		n := n + len(t)
		i := i + 1
	return n

def main() -> int:
	return churn()
`); got != 1 {
		t.Errorf("loop-local allocation: got %d rewindable loops, want 1", got)
	}
}

// THE dangerous shape. `t` does not outlive the function, so it is an arena
// candidate; it does outlive its iteration, because `keep` holds it. Rewinding
// would hand those bytes back out while keep still points at them.
func TestNoRewindWhenValueIsKeptOutsideTheLoop(t *testing.T) {
	if got := rewindableCount(t, `
def collect() -> int:
	let mut keep: list[list[int]] = []
	let mut i = 0
	while i < 10:
		let t = [i]
		keep.append(t)
		i := i + 1
	return len(keep)

def main() -> int:
	return collect()
`); got != 0 {
		t.Errorf("append into an outer list: got %d rewindable loops, want 0", got)
	}
}

// The same crash without storing anything: acc's backing array lives in the
// arena, and growing it allocates from the arena mid-iteration.
func TestNoRewindWhenAnOuterCollectionGrows(t *testing.T) {
	if got := rewindableCount(t, `
def build() -> int:
	let mut acc: list[int] = []
	let mut i = 0
	while i < 10:
		acc.append(i)
		i := i + 1
	return len(acc)

def main() -> int:
	return build()
`); got != 0 {
		t.Errorf("growing an outer list: got %d rewindable loops, want 0", got)
	}
}

// Reading an outer collection is fine and must not cost the loop its rewind.
// `outer` was allocated before the mark was taken, so rewinding to that mark
// cannot touch it, and reading creates no edge carrying anything back out.
func TestRewindWhenAnOuterCollectionIsMerelyRead(t *testing.T) {
	if got := rewindableCount(t, `
def readonly() -> int:
	let outer = [1, 2, 3]
	let mut i = 0
	let mut n = 0
	while i < 10:
		let t = [i]
		n := n + len(t) + len(outer)
		i := i + 1
	return n

def main() -> int:
	return readonly()
`); got != 1 {
		t.Errorf("reading an outer arena list: got %d rewindable loops, want 1", got)
	}
}

// Assigning the iteration's value to a variable declared outside the loop keeps
// it alive past the latch.
func TestNoRewindWhenValueEscapesToAnOuterLocal(t *testing.T) {
	if got := rewindableCount(t, `
def last() -> int:
	let mut best = [0]
	let mut i = 0
	while i < 10:
		let t = [i]
		best := t
		i := i + 1
	return len(best)

def main() -> int:
	return last()
`); got != 0 {
		t.Errorf("assigning to an outer local: got %d rewindable loops, want 0", got)
	}
}

// Returning from inside the loop makes the value escape the function outright,
// so it is not in the arena at all and there is nothing to rewind.
func TestNoRewindWhenNothingIsArenaAllocated(t *testing.T) {
	if got := rewindableCount(t, `
def find(n: int) -> list[int]:
	let mut i = 0
	while i < n:
		let t = [i]
		if i == 5:
			return t
		i := i + 1
	return []

def main() -> int:
	return len(find(10))
`); got != 0 {
		t.Errorf("value returned from the loop: got %d rewindable loops, want 0", got)
	}
}

// A loop that allocates nothing has nothing to reclaim.
func TestNoRewindForAllocationFreeLoop(t *testing.T) {
	if got := rewindableCount(t, `
def total(n: int) -> int:
	let mut i = 0
	let mut s = 0
	while i < n:
		s := s + i
		i := i + 1
	return s

def main() -> int:
	return total(10)
`); got != 0 {
		t.Errorf("allocation-free loop: got %d rewindable loops, want 0", got)
	}
}

// Nested loops are judged independently, and both qualify when each only
// touches what it allocated.
func TestRewindNestedLoopsBothQualify(t *testing.T) {
	if got := rewindableCount(t, `
def grid() -> int:
	let mut n = 0
	let mut i = 0
	while i < 10:
		let row = [i]
		let mut j = 0
		while j < 10:
			let cell = [j]
			n := n + len(cell) + len(row)
			j := j + 1
		i := i + 1
	return n

def main() -> int:
	return grid()
`); got != 2 {
		// The inner loop reads `row`, which the outer loop declared, and that is
		// harmless: `row` was allocated before the inner loop took its mark, so
		// the inner rewind cannot reach it.
		t.Errorf("nested loops: got %d rewindable loops, want 2", got)
	}
}

// An inner loop can qualify while the loop around it cannot. `keep` never
// leaves the function, so it stays in the arena and blocks the outer loop; the
// inner loop never mentions it and is judged on its own.
func TestRewindInnerLoopOnly(t *testing.T) {
	if got := rewindableCount(t, `
def mixed() -> int:
	let mut keep: list[list[int]] = []
	let mut n = 0
	let mut i = 0
	while i < 10:
		let held = [i]
		keep.append(held)
		let mut j = 0
		while j < 10:
			let tmp = [j]
			n := n + len(tmp)
			j := j + 1
		i := i + 1
	return n

def main() -> int:
	return mixed()
`); got != 1 {
		t.Errorf("inner-only: got %d rewindable loops, want 1", got)
	}
}

// The same program, except the kept list is returned, so `keep` escapes the
// function and everything flowing into it goes to the heap as well. Nothing
// arena-allocated actually crosses out of the outer loop here, so its rewind
// would in fact be safe -- but the rule blocks on the dependency edge itself
// rather than on what travels along it, so the outer loop is refused anyway and
// only the inner one qualifies. Deciding otherwise would mean following chains
// through heap containers, where a missed link is a use-after-free.
func TestRewindConservativeAboutFlowToAnEscapingCollection(t *testing.T) {
	if got := rewindableCount(t, `
def mixed() -> int:
	let mut keep: list[list[int]] = []
	let mut n = 0
	let mut i = 0
	while i < 10:
		let held = [i]
		keep.append(held)
		let mut j = 0
		while j < 10:
			let tmp = [j]
			n := n + len(tmp)
			j := j + 1
		i := i + 1
	return n + len(keep)

def main() -> int:
	return mixed()
`); got != 1 {
		t.Errorf("escaping outer collection: got %d rewindable loops, want 1 (inner only)", got)
	}
}

// for-loops get the same treatment as while-loops.
func TestRewindForLoop(t *testing.T) {
	if got := rewindableCount(t, `
def each() -> int:
	let mut n = 0
	for i in range(100):
		let t = [i, i]
		n := n + len(t)
	return n

def main() -> int:
	return each()
`); got != 1 {
		t.Errorf("for loop: got %d rewindable loops, want 1", got)
	}
}
