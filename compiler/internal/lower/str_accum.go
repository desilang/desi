package lower

import (
	"reflect"

	"github.com/desilang/desi/compiler/internal/ast"
)

// String accumulator analysis.
//
// A mutable string local qualifies as an "owned accumulator" when the
// compiler can prove every use of it borrows rather than retains the
// pointer. Qualifying variables are lowered with owned semantics:
//
//	let mut s = "lit"      →  s = __desi_str_new("lit")        (heap copy)
//	s := s + X             →  s = __desi_str_append_free(s, X) (realloc, frees old)
//	s := "lit"             →  free(s); s = __desi_str_new("lit")
//
// This turns the classic build-a-string loop from O(n^2) copying that
// leaks every intermediate value into amortized in-place appends with
// nothing leaked (except the final value, which follows the phase-3
// string-local rule).
//
// The analysis is conservative-by-default: any use it cannot prove
// borrowing — bare aliasing (`let t = s`), arguments to user functions,
// storage into collections or fields, lambda captures, `str(s)` (which
// is an identity bitcast, i.e. an alias), or any construct this walker
// doesn't recognize — disqualifies the variable, and it falls back to
// today's leak-safe lowering. Wrongly qualifying a variable would make
// the append FREE memory that something else still references, so every
// unknown defaults to "no".

// strAccumCandidates returns the set of variable names in fd that
// qualify for owned-accumulator lowering.
func strAccumCandidates(fd *ast.FuncDecl) map[string]bool {
	if fd == nil || fd.Body == nil {
		return nil
	}

	// Pass 1: collect `let mut NAME = <string literal>` declarations.
	candidates := map[string]bool{}
	var collect func(n ast.Node)
	collect = func(n ast.Node) {
		if let, ok := n.(*ast.LetStmt); ok {
			if let.Mutable && let.Name.Name != "" {
				if _, isLit := let.Value.(*ast.StrLit); isLit {
					candidates[let.Name.Name] = true
				}
			}
		}
	}
	walkNodes(fd.Body, collect)
	if len(candidates) == 0 {
		return nil
	}

	// Pass 2: strike any candidate with a non-borrowing use.
	for name := range candidates {
		if !strAccumUsesOK(fd.Body, name) {
			delete(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}

// isStrAccumAppend reports whether an assignment statement has the shape
// `name := name + X` with X not referencing name (self-referencing
// suffixes would read the buffer the append just freed).
func isStrAccumAppend(s *ast.AssignStmt, name string) (suffix ast.Expr, ok bool) {
	if len(s.LHS) != 1 || len(s.RHS) != 1 {
		return nil, false
	}
	lhs, ok2 := s.LHS[0].(*ast.Ident)
	if !ok2 || lhs.Name != name {
		return nil, false
	}
	bin, ok2 := s.RHS[0].(*ast.BinaryExpr)
	if !ok2 || bin.Op != "+" {
		return nil, false
	}
	l, ok2 := bin.Lhs.(*ast.Ident)
	if !ok2 || l.Name != name {
		return nil, false
	}
	if mentionsName(bin.Rhs, name) {
		return nil, false
	}
	return bin.Rhs, true
}

// isStrAccumReset reports whether an assignment is `name := <StrLit>`.
func isStrAccumReset(s *ast.AssignStmt, name string) bool {
	if len(s.LHS) != 1 || len(s.RHS) != 1 {
		return false
	}
	lhs, ok := s.LHS[0].(*ast.Ident)
	if !ok || lhs.Name != name {
		return false
	}
	_, isLit := s.RHS[0].(*ast.StrLit)
	return isLit
}

// strAccumUsesOK walks the function body and verifies every occurrence
// of name is a borrowing use or a rewritable assignment.
func strAccumUsesOK(body *ast.Block, name string) bool {
	ok := true
	var walkStmt func(n ast.Node)

	// exprBorrowsOnly reports whether every occurrence of name inside e
	// is under a borrowing operator (concat/compare operand, f-string
	// part, len(...) argument, index base, direct print argument).
	var exprBorrowsOnly func(e ast.Expr) bool
	// operand allows a bare `name` (the enclosing operator borrows it).
	operand := func(e ast.Expr) bool {
		if id, isID := e.(*ast.Ident); isID && id.Name == name {
			return true
		}
		return exprBorrowsOnly(e)
	}
	exprBorrowsOnly = func(e ast.Expr) bool {
		if e == nil || !mentionsName(e, name) {
			return true
		}
		switch x := e.(type) {
		case *ast.Ident:
			// A bare alias in a non-borrowing position.
			return x.Name != name
		case *ast.BinaryExpr:
			// string_concat / comparisons copy or read their operands.
			return operand(x.Lhs) && operand(x.Rhs)
		case *ast.UnaryExpr:
			return operand(x.X)
		case *ast.IndexExpr:
			// s[i] reads; the index itself must also be safe.
			return operand(x.X) && exprBorrowsOnly(x.Idx)
		case *ast.FString:
			// f-strings copy into a fresh buffer.
			for _, p := range x.Parts {
				if fe, isFE := p.(*ast.FStringExpr); isFE {
					if !operand(fe.X) {
						return false
					}
				} else if pe, isExpr := p.(ast.Expr); isExpr {
					if !operand(pe) {
						return false
					}
				}
			}
			return true
		case *ast.CallExpr:
			// Only builtins known to borrow. str(s) is EXCLUDED: it is an
			// identity bitcast for strings, i.e. an alias.
			callee, isID := x.Callee.(*ast.Ident)
			if !isID {
				return false
			}
			switch callee.Name {
			case "print", "len":
				for _, a := range x.Args {
					if !operand(a) {
						return false
					}
				}
				return true
			}
			return false
		default:
			return false
		}
	}

	walkStmt = func(n ast.Node) {
		if !ok || n == nil {
			return
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			if _, isAppend := isStrAccumAppend(s, name); isAppend {
				return // rewritable — suffix already checked s-free
			}
			if isStrAccumReset(s, name) {
				return // rewritable
			}
			// Any other assignment touching name (either side) is out.
			for _, l := range s.LHS {
				if mentionsName(l, name) {
					ok = false
					return
				}
			}
			for _, r := range s.RHS {
				if !exprBorrowsOnly(r) {
					ok = false
					return
				}
			}
		case *ast.LetStmt:
			// The candidate's own literal init is fine; any OTHER let
			// whose init mentions name must be a borrowing expression
			// (a bare `let t = s` alias disqualifies).
			if s.Name.Name == name {
				return
			}
			if !exprBorrowsOnly(s.Value) {
				ok = false
			}
		case *ast.ReturnStmt:
			// Returning the accumulator hands ownership out at exit —
			// no appends run afterwards on this path.
			if id, isID := s.Value.(*ast.Ident); isID && id.Name == name {
				return
			}
			if !exprBorrowsOnly(s.Value) {
				ok = false
			}
		case *ast.ExprStmt:
			if !exprBorrowsOnly(s.Expr) {
				ok = false
			}
		case *ast.IfStmt:
			if !exprBorrowsOnly(s.Cond) {
				ok = false
				return
			}
			for _, elif := range s.Elifs {
				if !exprBorrowsOnly(elif.Cond) {
					ok = false
					return
				}
			}
		case *ast.WhileStmt:
			if !exprBorrowsOnly(s.Cond) {
				ok = false
			}
		case *ast.ForStmt:
			if !exprBorrowsOnly(s.Iter) {
				ok = false
			}
		case *ast.Block:
			// container — children visited by the walker
		default:
			// Unrecognized statement kind (match, using, try, defer,
			// lambda-bearing constructs, ...): if it mentions the name
			// at all, refuse. Conservative-by-default.
			if nodeMentionsName(n, name) {
				ok = false
			}
			return
		}
	}

	// Visit statements; walkNodes descends generically, so nested blocks
	// (if/while bodies) reach walkStmt too.
	walkNodes(body, func(n ast.Node) {
		if _, isStmt := n.(ast.Stmt); isStmt {
			walkStmt(n)
		}
	})
	return ok
}

// mentionsName reports whether expression e contains an Ident named name.
func mentionsName(e ast.Expr, name string) bool {
	return nodeMentionsName(e, name)
}

// nodeMentionsName is a generic reflective scan for an Ident with the
// given name anywhere beneath n.
func nodeMentionsName(n ast.Node, name string) bool {
	found := false
	walkNodes(n, func(x ast.Node) {
		if id, isID := x.(*ast.Ident); isID && id.Name == name {
			found = true
		}
	})
	return found
}

// walkNodes is a complete generic AST traversal (reflection-based, like
// check/escape.go's): every struct field, slice element, and value
// struct containing an ast.Node is visited. Unknown node kinds cannot
// be silently skipped — this analysis's safety depends on seeing every
// occurrence.
func walkNodes(n ast.Node, fn func(ast.Node)) {
	if n == nil {
		return
	}
	rv := reflect.ValueOf(n)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return
	}
	fn(n)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	walkNodeChildren(rv, fn)
}

func walkNodeChildren(rv reflect.Value, fn func(ast.Node)) {
	switch rv.Kind() {
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			visitNodeChild(rv.Field(i), fn)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			visitNodeChild(rv.Index(i), fn)
		}
	}
}

func visitNodeChild(fv reflect.Value, fn func(ast.Node)) {
	if !fv.IsValid() || !fv.CanInterface() {
		return
	}
	switch fv.Kind() {
	case reflect.Interface, reflect.Ptr:
		if fv.IsNil() {
			return
		}
		if n, isNode := fv.Interface().(ast.Node); isNode {
			walkNodes(n, fn)
			return
		}
		if fv.Kind() == reflect.Ptr {
			walkNodeChildren(fv.Elem(), fn)
		} else {
			visitNodeChild(fv.Elem(), fn)
		}
	case reflect.Struct:
		if fv.CanAddr() {
			if n, isNode := fv.Addr().Interface().(ast.Node); isNode {
				walkNodes(n, fn)
				return
			}
		}
		walkNodeChildren(fv, fn)
	case reflect.Slice, reflect.Array:
		for i := 0; i < fv.Len(); i++ {
			visitNodeChild(fv.Index(i), fn)
		}
	}
}
