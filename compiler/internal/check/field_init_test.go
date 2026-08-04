package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
)

// fullyInit checks src and reports which classes the constructor analysis
// proved complete. Sources here are written with tabs, matching the examples.
func fullyInit(t *testing.T, src string) map[string]bool {
	t.Helper()
	mod, pdiags := parse.ParseFile("<mem>", []byte(strings.TrimLeft(src, "\n")))
	if len(pdiags) != 0 {
		t.Fatalf("parse diags: %+v", pdiags)
	}
	_, info := Check(mod)
	if info == nil {
		t.Fatal("no check info")
	}
	return info.FullyInitClasses
}

func TestFieldInitProvesCompleteConstructor(t *testing.T) {
	got := fullyInit(t, `
class Point:
	pub mut x: int
	pub mut y: int
	pub def __new__(self, a: int, b: int):
		self.x := a
		self.y := b

def main() -> int:
	let p = Point(1, 2)
	return p.x
`)
	if !got["Point"] {
		t.Error("a constructor that assigns every field was not proven complete")
	}
}

func TestFieldInitRejectsIncompleteConstructor(t *testing.T) {
	got := fullyInit(t, `
class Partial:
	pub mut x: int
	pub mut y: int
	pub def __new__(self, a: int):
		self.x := a

def main() -> int:
	let p = Partial(1)
	return p.x
`)
	if got["Partial"] {
		t.Error("a constructor that leaves y unwritten was claimed complete")
	}
}

// Both arms assign, so the field is written whichever way the branch goes.
func TestFieldInitAcceptsIfElseWhenBothArmsAssign(t *testing.T) {
	got := fullyInit(t, `
class Branchy:
	pub mut x: int
	pub def __new__(self, a: int):
		if a > 0:
			self.x := a
		else:
			self.x := 0

def main() -> int:
	let b = Branchy(1)
	return b.x
`)
	if !got["Branchy"] {
		t.Error("an if/else assigning in both arms was not proven complete")
	}
}

// The missing else is itself a path, and it assigns nothing.
func TestFieldInitRejectsIfWithoutElse(t *testing.T) {
	got := fullyInit(t, `
class NoElse:
	pub mut x: int
	pub def __new__(self, a: int):
		if a > 0:
			self.x := a

def main() -> int:
	let n = NoElse(1)
	return n.x
`)
	if got["NoElse"] {
		t.Error("an if with no else was claimed complete")
	}
}

// An early return skips the later assignment.
func TestFieldInitRejectsEarlyReturn(t *testing.T) {
	got := fullyInit(t, `
class Early:
	pub mut x: int
	pub mut y: int
	pub def __new__(self, a: int):
		self.x := a
		if a < 0:
			return
		self.y := a

def main() -> int:
	let e = Early(1)
	return e.x
`)
	if got["Early"] {
		t.Error("a constructor with an early return before assigning y was claimed complete")
	}
}

// A loop body's assignments do not count, but a return inside one still leaves
// the constructor and has to be seen.
func TestFieldInitRejectsReturnInsideLoop(t *testing.T) {
	got := fullyInit(t, `
class LoopExit:
	pub mut x: int
	pub mut y: int
	pub def __new__(self, n: int):
		self.x := n
		let mut i = 0
		while i < n:
			if i == 2:
				return
			i := i + 1
		self.y := n

def main() -> int:
	let l = LoopExit(5)
	return l.x
`)
	if got["LoopExit"] {
		t.Error("a return inside a loop body, leaving y unwritten, was not seen as an exit")
	}
}

// Same hole, reached through a try body rather than a loop.
func TestFieldInitRejectsReturnInsideTry(t *testing.T) {
	got := fullyInit(t, `
class TryExit:
	pub mut x: int
	pub mut y: int
	pub def __new__(self, n: int):
		self.x := n
		try:
			if n < 0:
				return
		except:
			pass
		self.y := n

def main() -> int:
	let e = TryExit(5)
	return e.x
`)
	if got["TryExit"] {
		t.Error("a return inside a try body, leaving y unwritten, was not seen as an exit")
	}
}

// A loop body may run zero times, so nothing it assigns is guaranteed.
func TestFieldInitRejectsAssignmentOnlyInLoop(t *testing.T) {
	got := fullyInit(t, `
class Looped:
	pub mut x: int
	pub def __new__(self, n: int):
		let mut i = 0
		while i < n:
			self.x := i
			i := i + 1

def main() -> int:
	let l = Looped(3)
	return l.x
`)
	if got["Looped"] {
		t.Error("a field assigned only inside a loop was claimed complete")
	}
}

// No constructor at all is a supported shape: zero initialisation is the
// defined behaviour there, so the class must keep it rather than be diagnosed.
func TestFieldInitLeavesConstructorlessClassAlone(t *testing.T) {
	got := fullyInit(t, `
class Bare:
	pub mut x: int

def main() -> int:
	let mut b = Bare()
	b.x := 5
	return b.x
`)
	if got["Bare"] {
		t.Error("a class with no __new__ was claimed complete; its fields are never assigned")
	}
}

// Every overload has to be complete — one incomplete constructor means the
// zeroing has to stay for all of them, since they share the allocation site.
func TestFieldInitRequiresEveryOverloadComplete(t *testing.T) {
	got := fullyInit(t, `
class Two:
	pub mut x: int
	pub mut y: int
	pub def __new__(self, a: int, b: int):
		self.x := a
		self.y := b
	pub def __new__(self, a: int):
		self.x := a

def main() -> int:
	let t2 = Two(1, 2)
	return t2.x
`)
	if got["Two"] {
		t.Error("a class with one incomplete overload was claimed complete")
	}
}

// A derived class's layout carries the base's fields too, which its own __new__
// does not assign. Those keep the zeroing.
func TestFieldInitSkipsDerivedClasses(t *testing.T) {
	got := fullyInit(t, `
class Base:
	pub mut a: int
	pub def __new__(self, a: int):
		self.a := a

class Derived(Base):
	pub mut b: int
	pub def __new__(self, b: int):
		self.b := b

def main() -> int:
	let d = Derived(1)
	return d.b
`)
	if got["Derived"] {
		t.Error("a derived class was proven complete without accounting for base fields")
	}
}

// The receiver is whatever the first parameter is named, not literally "self".
func TestFieldInitFollowsRenamedReceiver(t *testing.T) {
	got := fullyInit(t, `
class Renamed:
	pub mut x: int
	pub def __new__(this, a: int):
		this.x := a

def main() -> int:
	let r = Renamed(1)
	return r.x
`)
	if !got["Renamed"] {
		t.Error("a constructor using a receiver named other than self was not followed")
	}
}
