package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// Definite assignment for class constructors.
//
// Every instance is zeroed before `__new__` runs, so a field the constructor
// forgets reads back as 0 (or null) instead of as garbage. That is a real
// guarantee and the drop paths depend on it — they null-check pointer fields
// before freeing. It is also a memset on every single allocation, and for the
// overwhelming majority of constructors it is dead work: `__new__` assigns
// every field itself, so the zeroing is immediately overwritten.
//
// This pass finds those constructors. When every `__new__` on a class assigns
// every field on every path that can reach the end of the constructor, the
// class is recorded in Info.FullyInitClasses and lowering skips the zeroing
// entirely.
//
// The analysis is deliberately one-directional: it may fail to notice that a
// constructor is complete, and the only cost is a memset that was going to
// happen anyway. It must never claim a constructor is complete when it is not,
// because then a field would be read before it was written. Every case the
// analysis cannot follow — a helper method that assigns fields, a loop, a
// match — therefore reads as "not proven".
//
// Classes with no `__new__` at all are untouched. `Point()` with bare field
// declarations and no constructor is a documented Desi shape, and zero
// initialisation is its defined behaviour, not an oversight to be diagnosed.

// runFieldInitAnalysis records every class whose constructors provably leave no
// field unwritten.
func runFieldInitAnalysis(mod *ast.Module, info *Info) {
	if mod == nil || info == nil {
		return
	}
	for _, d := range mod.Decls {
		cd, ok := d.(*ast.ClassDecl)
		if !ok {
			continue
		}
		analyzeClassFieldInit(cd, info)
		// Nested classes are declared inside the parent but constructed by the
		// same code path, so they need the same treatment.
		for _, n := range cd.Nested {
			analyzeClassFieldInit(n, info)
		}
	}
}

func analyzeClassFieldInit(cd *ast.ClassDecl, info *Info) {
	if cd == nil || len(cd.Fields) == 0 {
		return
	}
	// A derived class's layout includes the base's fields, which its own
	// `__new__` generally cannot reach. Rather than model that, leave inherited
	// layouts to the zeroing.
	if len(cd.Bases) > 0 {
		return
	}

	want := make(map[string]bool, len(cd.Fields))
	for _, f := range cd.Fields {
		if f != nil && f.Name.Name != "" {
			want[f.Name.Name] = true
		}
	}
	if len(want) == 0 {
		return
	}

	ctors := 0
	for _, m := range cd.Methods {
		if m == nil || m.Name.Name != "__new__" || m.Body == nil {
			continue
		}
		ctors++
		// The receiver is whatever the first parameter is called. It is `self`
		// by convention, but the convention is not enforced here.
		if len(m.Params) == 0 {
			return
		}
		self := m.Params[0].Name.Name
		if self == "" {
			return
		}
		a := &fieldInitAnalysis{self: self, fields: want}
		out, left := a.block(m.Body, map[string]bool{})
		if !left {
			// Falling off the end is an exit too.
			a.exits = append(a.exits, out)
		}
		// The constructor is complete only if every way out of it has written
		// every field — an early return counts, and so does a raise, since a
		// half-built instance can still reach the drop path.
		got := intersectAll(a.exits)
		for name := range want {
			if !got[name] {
				return // this overload can leave a field unwritten
			}
		}
	}
	if ctors == 0 {
		return // no constructor: zero initialisation is the contract
	}
	if info.FullyInitClasses == nil {
		info.FullyInitClasses = map[string]bool{}
	}
	info.FullyInitClasses[cd.Name.Name] = true
}

type fieldInitAnalysis struct {
	self   string            // name of the receiver parameter
	fields map[string]bool   // the class's own field names
	exits  []map[string]bool // fields assigned at each point control leaves __new__
}

// block walks a statement list and returns the fields definitely assigned once
// it finishes, plus whether control always leaves the block rather than falling
// off the end.
func (a *fieldInitAnalysis) block(b *ast.Block, in map[string]bool) (map[string]bool, bool) {
	cur := copySet(in)
	if b == nil {
		return cur, false
	}
	for _, st := range b.Stmts {
		var left bool
		cur, left = a.stmt(st, cur)
		if left {
			// Nothing after this runs, so nothing after it can assign.
			return cur, true
		}
	}
	return cur, false
}

func (a *fieldInitAnalysis) stmt(st ast.Stmt, in map[string]bool) (map[string]bool, bool) {
	switch x := st.(type) {
	case *ast.AssignStmt:
		out := copySet(in)
		for _, lhs := range x.LHS {
			if name, ok := a.selfField(lhs); ok {
				out[name] = true
			}
		}
		return out, false

	case *ast.IfStmt:
		// A field counts as assigned after the statement only if every route
		// through it assigns the field. A missing `else` is itself a route that
		// assigns nothing, so without one the statement can only preserve what
		// was already assigned.
		thenOut, thenLeft := a.block(x.Then, in)
		outs := []map[string]bool{}
		allLeave := thenLeft
		if !thenLeft {
			outs = append(outs, thenOut)
		}
		for i := range x.Elifs {
			o, left := a.block(x.Elifs[i].Body, in)
			if !left {
				outs = append(outs, o)
			}
			allLeave = allLeave && left
		}
		if x.Else != nil {
			o, left := a.block(x.Else, in)
			if !left {
				outs = append(outs, o)
			}
			allLeave = allLeave && left
		} else {
			// The implicit empty else falls through having assigned nothing new.
			outs = append(outs, copySet(in))
			allLeave = false
		}
		if allLeave {
			// Every arm returned or raised; the statement never falls through.
			return copySet(in), true
		}
		return intersectAll(outs), false

	case *ast.TryStmt:
		// The try body can fault at any statement, so nothing it assigns is
		// guaranteed. Only `finally` runs no matter what. Both bodies are still
		// walked, because a return inside either is a way out of the
		// constructor and has to be recorded as one.
		a.block(x.Body, in)
		if x.Except != nil {
			a.block(x.Except, in)
		}
		out := copySet(in)
		if x.Finally != nil {
			out, _ = a.block(x.Finally, out)
		}
		return out, false

	case *ast.ReturnStmt, *ast.RaiseStmt:
		// Control leaves the constructor here, carrying whatever has been
		// assigned so far. Raising counts as well: the caller never receives the
		// instance, but the half-built allocation can still reach a drop path,
		// and that path reads pointer fields to free them.
		out := copySet(in)
		a.exits = append(a.exits, out)
		return out, true

	case *ast.BreakStmt, *ast.ContinueStmt:
		// Leaves the enclosing block, but not the constructor.
		return copySet(in), true

	case *ast.WhileStmt:
		// A loop body may run zero times, so nothing it assigns is guaranteed.
		// Walk it anyway: a return inside the loop leaves the constructor and
		// must be recorded as an exit.
		a.block(x.Body, in)
		return copySet(in), false

	case *ast.ForStmt:
		a.block(x.Body, in)
		return copySet(in), false
	}

	// Everything else — expression statements, lets, asserts, spawns, defers,
	// match in statement position — is not followed. Assignments made through
	// them simply go unnoticed, which costs a memset and nothing else.
	return copySet(in), false
}

// selfField reports whether an assignment target is `self.<field>` naming one of
// this class's own fields.
func (a *fieldInitAnalysis) selfField(e ast.Expr) (string, bool) {
	fe, ok := e.(*ast.FieldExpr)
	if !ok {
		return "", false
	}
	id, ok := fe.X.(*ast.Ident)
	if !ok || id.Name != a.self {
		return "", false
	}
	if !a.fields[fe.Name.Name] {
		return "", false
	}
	return fe.Name.Name, true
}

func copySet(s map[string]bool) map[string]bool {
	out := make(map[string]bool, len(s))
	for k, v := range s {
		if v {
			out[k] = true
		}
	}
	return out
}

func intersectAll(sets []map[string]bool) map[string]bool {
	if len(sets) == 0 {
		return map[string]bool{}
	}
	out := copySet(sets[0])
	for _, s := range sets[1:] {
		for k := range out {
			if !s[k] {
				delete(out, k)
			}
		}
	}
	return out
}
