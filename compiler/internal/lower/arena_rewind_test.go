package lower_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
	"github.com/desilang/desi/compiler/internal/parse"
)

// Whether a loop reuses its arena bytes is decided in check/escape_loop.go and
// tested there. These tests cover the other half: that the decision reaches the
// generated code, and — for the shapes where rewinding would hand back memory
// something still points at — that it does not.
//
// They read the lowered calls rather than running the program on purpose. A
// use-after-free is not something a behaviour test reliably catches; it catches
// it on the run where the reused bytes happen to have been overwritten, and not
// on the run where they happen not to have been.

// arenaCalls lowers the named function from src and counts the arena calls in
// it, keyed by runtime function name.
func arenaCalls(t *testing.T, fnName, src string) map[string]int {
	t.Helper()
	source := []byte(strings.TrimLeft(src, "\n"))
	mod, pdiags := parse.ParseFile("<mem>", source)
	if len(pdiags) != 0 {
		t.Fatalf("parse diags: %+v", pdiags)
	}
	diags, info := check.Check(mod)
	if len(diags) != 0 {
		t.Fatalf("check diags: %+v", diags)
	}

	var fd *ast.FuncDecl
	for _, d := range mod.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == fnName {
			fd = f
			break
		}
	}
	if fd == nil {
		t.Fatalf("no function named %q in the source", fnName)
	}

	fn := lower.LowerFuncFromDecl(fd, info, source, map[string]bool{})
	if fn == nil {
		t.Fatal("lowering produced no function")
	}

	counts := map[string]int{}
	for _, blk := range fn.Blocks {
		if blk == nil {
			continue
		}
		for _, st := range blk.Stmts {
			if c, ok := st.(*hir.Call); ok && strings.HasPrefix(c.Fn, "__arena_") {
				counts[c.Fn]++
			}
		}
	}
	return counts
}

// A loop-local allocation gets the full bracket: one mark before the loop, a
// rewind at the latch, and a release on the way out.
func TestLoopLocalAllocationIsBracketed(t *testing.T) {
	got := arenaCalls(t, "churn", `
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
`)
	for _, want := range []string{"__arena_mark", "__arena_rewind", "__arena_release"} {
		if got[want] != 1 {
			t.Errorf("%s: emitted %d times, want 1 (all calls: %v)", want, got[want], got)
		}
	}
}

// The shape that must never rewind: `t` is arena-allocated and does not outlive
// the function, but `keep` holds it past the end of the iteration.
func TestValueKeptOutsideTheLoopGetsNoRewind(t *testing.T) {
	got := arenaCalls(t, "collect", `
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
`)
	if got["__arena_rewind"] != 0 || got["__arena_mark"] != 0 {
		t.Errorf("a loop whose value is kept outside it was bracketed anyway: %v", got)
	}
}

// Storing the iteration's value into a variable declared outside the loop is
// the same hazard by a different route.
func TestValueStoredToOuterLocalGetsNoRewind(t *testing.T) {
	got := arenaCalls(t, "last", `
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
`)
	if got["__arena_rewind"] != 0 || got["__arena_mark"] != 0 {
		t.Errorf("a loop assigning to an outer local was bracketed anyway: %v", got)
	}
}

// A list that grows must stay on the heap, so there is no arena in the picture
// at all and nothing to bracket.
func TestGrowingCollectionUsesNoArena(t *testing.T) {
	got := arenaCalls(t, "build", `
def build() -> int:
	let mut acc: list[int] = []
	let mut i = 0
	while i < 10:
		acc.append(i)
		i := i + 1
	return len(acc)

def main() -> int:
	return build()
`)
	if got["__arena_rewind"] != 0 || got["__arena_mark"] != 0 {
		t.Errorf("a loop growing an outer list was bracketed anyway: %v", got)
	}
}

// Nested loops each get their own mark, and each releases it. Pairing matters:
// the runtime keeps a stack of marks, and a mark that is never released would
// leave the arena unable to reclaim anything past it.
func TestNestedLoopsAreEachBracketed(t *testing.T) {
	got := arenaCalls(t, "grid", `
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
`)
	if got["__arena_mark"] != 2 {
		t.Errorf("__arena_mark: emitted %d times, want 2 (all calls: %v)", got["__arena_mark"], got)
	}
	if got["__arena_mark"] != got["__arena_release"] {
		t.Errorf("marks and releases must pair: %v", got)
	}
	if got["__arena_rewind"] != 2 {
		t.Errorf("__arena_rewind: emitted %d times, want 2 (all calls: %v)", got["__arena_rewind"], got)
	}
}

// A loop that allocates nothing should not pay for a mark it has no use for.
func TestAllocationFreeLoopIsNotBracketed(t *testing.T) {
	got := arenaCalls(t, "total", `
def total(n: int) -> int:
	let mut i = 0
	let mut s = 0
	while i < n:
		s := s + i
		i := i + 1
	return s

def main() -> int:
	return total(10)
`)
	if len(got) != 0 {
		t.Errorf("an allocation-free loop emitted arena calls: %v", got)
	}
}
