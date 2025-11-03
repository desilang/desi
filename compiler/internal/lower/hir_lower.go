package lower

import (
	"bytes"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
)

// LowerBlock lowers an AST block into a single HIR function named `name`.
// It places deterministic Drops at scope ends and before any early return.
func LowerBlock(name string, blk *ast.Block) *hir.Func {
	b := hir.NewFunc(name)
	ls := &lowerState{
		b:      b,
		scopes: []*scope{{}}, // push root scope
	}
	ls.lowerBlock(blk)
	return b.Func()
}

type lowerState struct {
	b      *hir.Builder
	scopes []*scope // stack
}

type scope struct {
	locals []string // in declaration order
	defers []hir.Value
}

func (ls *lowerState) push() { ls.scopes = append(ls.scopes, &scope{}) }
func (ls *lowerState) pop() *scope {
	last := ls.scopes[len(ls.scopes)-1]
	ls.scopes = ls.scopes[:len(ls.scopes)-1]
	return last
}
func (ls *lowerState) cur() *scope { return ls.scopes[len(ls.scopes)-1] }

// ---- lowering ----

func (ls *lowerState) lowerBlock(blk *ast.Block) {
	for _, st := range blk.Stmts {
		ls.lowerStmt(st)
	}
	// End-of-root-block finalization (only for outermost scope).
	if len(ls.scopes) == 1 {
		ls.emitScopeDrops(ls.cur())
	}
}

func (ls *lowerState) lowerStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.LetStmt:
		// shadowing in same scope drops previous
		if ls.hasLocal(s.Name.Name) {
			ls.b.Emit(&hir.Drop{Val: hir.Var{Name: s.Name.Name}})
			ls.removeLocal(s.Name.Name)
		}
		ls.cur().locals = append(ls.cur().locals, s.Name.Name)
		var init hir.Value
		if s.Value != nil {
			init = ls.lowerExpr(s.Value)
		}
		ls.b.Emit(&hir.Let{Name: s.Name.Name, Init: init})

	case *ast.AssignStmt:
		if len(s.LHS) == 1 && len(s.RHS) == 1 {
			lhs := ls.lowerLValue(s.LHS[0])
			rhs := ls.lowerExpr(s.RHS[0])
			ls.b.Emit(&hir.Assign{LHS: lhs, RHS: rhs})
		}

	case *ast.ExprStmt:
		_ = ls.lowerExpr(s.Expr) // materialize for side effects if needed

	case *ast.ReturnStmt:
		// Before returning, run defers and drop locals from all open scopes (inner→outer).
		ls.emitAllDefersAndDrops()
		var v hir.Value
		if s.Value != nil {
			v = ls.lowerExpr(s.Value)
		}
		ls.b.Emit(&hir.Ret{Val: v})

	case *ast.IfStmt:
		cond := ls.lowerExpr(s.Cond)
		thenBlk := hir.NewBlock("then")
		elseBlk := (*hir.Block)(nil)
		// Lower bodies in isolated lowering states sharing the same builder,
		// but with pushed scopes so drops are contained.
		ls.push()
		oldCur := ls.b.Block()
		ls.b.SetBlock(thenBlk)
		ls.lowerBlock(s.Then)
		ls.emitScopeDrops(ls.pop()) // finalize then-scope
		ls.b.SetBlock(oldCur)       // restore
		if s.Else != nil {
			elseBlk = hir.NewBlock("else")
			ls.push()
			ls.b.SetBlock(elseBlk)
			ls.lowerBlock(s.Else)
			ls.emitScopeDrops(ls.pop())
			ls.b.SetBlock(oldCur)
		}
		ls.b.Emit(&hir.If{Cond: cond, Then: thenBlk, Else: elseBlk})

	case *ast.WhileStmt:
		cond := ls.lowerExpr(s.Cond)
		bodyBlk := hir.NewBlock("while")
		ls.push()
		oldCur := ls.b.Block()
		ls.b.SetBlock(bodyBlk)
		ls.lowerBlock(s.Body)
		ls.emitScopeDrops(ls.pop())
		ls.b.SetBlock(oldCur)
		ls.b.Emit(&hir.While{Cond: cond, Body: bodyBlk})

	case *ast.UsingStmt:
		// using X [= init]: body
		ls.push()
		ident := ls.nameOf(s.Bind)
		if ident != "" {
			// introduce the handle in the scope so it participates in end-of-scope drops
			ls.cur().locals = append(ls.cur().locals, ident)
			if s.Init != nil {
				init := ls.lowerExpr(s.Init)
				ls.b.Emit(&hir.Let{Name: ident, Init: init})
			} else {
				ls.b.Emit(&hir.Let{Name: ident})
			}
			// M7A: model as a single Drop(handle) defer
			ls.cur().defers = append(ls.cur().defers, hir.Var{Name: ident})
		}
		ls.lowerBlock(s.Body)
		sc := ls.pop()
		ls.emitScopeDrops(sc)

	case *ast.DeferStmt:
		// For M7A, capture a generic "defer drop <target>" if shape is simple.
		if ce := s.Call; ce != nil {
			// naive: if call is "__close(x)" treat as drop x
			if id, ok := ce.Func.(*ast.Ident); ok && id.Name == "__close" && len(ce.Args) == 1 {
				if v := ls.valueOf(ce.Args[0]); v != nil {
					ls.cur().defers = append(ls.cur().defers, v)
				}
			}
		}
	default:
		// other statements ignored for M7A
	}
}

func (ls *lowerState) lowerExpr(e ast.Expr) hir.Value {
	switch e := e.(type) {
	case *ast.IntLit:
		return hir.ConstInt{Text: e.Text}
	case *ast.BoolLit:
		return hir.ConstBool{Value: e.Value}
	case *ast.StrLit:
		return hir.ConstStr{Text: e.Value}
	case *ast.Ident:
		return hir.Var{Name: e.Name}
	case *ast.CallExpr:
		fnName := ls.calleeName(e.Func)
		args := make([]hir.Value, 0, len(e.Args))
		for _, a := range e.Args {
			args = append(args, ls.lowerExpr(a))
		}
		dst := ls.b.FreshTemp("t")
		ls.b.Emit(&hir.Call{Fn: fnName, Args: args, Dst: dst})
		return dst
	case *ast.FieldExpr:
		// treat as qualified name reference e.g., 'arena.alloc' when used as callee
		return hir.Var{Name: ls.fieldName(e)}
	default:
		// fallback string via AST printer if ever needed
		var buf bytes.Buffer
		ast.Print(&buf, e)
		return hir.Var{Name: buf.String()}
	}
}

func (ls *lowerState) lowerLValue(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	default:
		var buf bytes.Buffer
		ast.Print(&buf, e)
		return buf.String()
	}
}

func (ls *lowerState) nameOf(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func (ls *lowerState) fieldName(e *ast.FieldExpr) string {
	// flatten a.b.c into "a.b.c"
	names := []string{}
	var walk func(ast.Expr)
	walk = func(x ast.Expr) {
		switch x := x.(type) {
		case *ast.Ident:
			names = append(names, x.Name)
		case *ast.FieldExpr:
			walk(x.X)
			names = append(names, x.Name.Name)
		}
	}
	walk(e)
	return stringsJoin(names, ".")
}

func stringsJoin(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}

// emit drops for one scope (LIFO over locals, then defers LIFO)
func (ls *lowerState) emitScopeDrops(sc *scope) {
	// First drop locals in reverse declaration order.
	for i := len(sc.locals) - 1; i >= 0; i-- {
		ls.b.Emit(&hir.Drop{Val: hir.Var{Name: sc.locals[i]}})
	}
	// Then run defers in LIFO order.
	for i := len(sc.defers) - 1; i >= 0; i-- {
		ls.b.Emit(&hir.Drop{Val: sc.defers[i]})
	}
}

func (ls *lowerState) emitAllDefersAndDrops() {
	// Emit all pending defers and locals from inner-most to outer-most.
	for i := len(ls.scopes) - 1; i >= 0; i-- {
		ls.emitScopeDrops(ls.scopes[i])
	}
}

func (ls *lowerState) hasLocal(name string) bool {
	sc := ls.cur()
	for _, n := range sc.locals {
		if n == name {
			return true
		}
	}
	return false
}

func (ls *lowerState) removeLocal(name string) {
	sc := ls.cur()
	for i, n := range sc.locals {
		if n == name {
			copy(sc.locals[i:], sc.locals[i+1:])
			sc.locals = sc.locals[:len(sc.locals)-1]
			return
		}
	}
}

// Helpers used in DeferStmt lowering.
func (ls *lowerState) valueOf(e ast.Expr) hir.Value {
	switch x := e.(type) {
	case *ast.Ident:
		return hir.Var{Name: x.Name}
	default:
		return nil
	}
}
