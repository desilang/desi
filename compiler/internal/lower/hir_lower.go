package lower

import (
	"bytes"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// LowerBlock lowers an AST block into a single HIR function named `name`.
func LowerBlock(name string, blk *ast.Block) *hir.Func {
	return LowerBlockWithInfo(name, blk, nil)
}

// LowerBlockWithInfo is like LowerBlock but can consult type info (from checker)
// to specialize drops for rc/arc handles into DecRef, and to recognize simple moves.
func LowerBlockWithInfo(name string, blk *ast.Block, info *check.Info) *hir.Func {
	b := hir.NewFunc(name)
	ls := &lowerState{
		b:          b,
		scopes:     []*scope{{locals: []string{}, rcLike: map[string]bool{}, moved: map[string]bool{}}}, // root
		terminated: false,
		info:       info,
	}
	ls.lowerBlock(blk)
	return b.Func()
}

type lowerState struct {
	b          *hir.Builder
	scopes     []*scope // stack
	terminated bool     // set once a return is emitted
	info       *check.Info
}

type scope struct {
	locals []string        // in declaration order
	rcLike map[string]bool // locals that are rc/arc
	moved  map[string]bool // locals that have been moved out; skip drop
	defers []hir.Value
}

func (ls *lowerState) push() {
	ls.scopes = append(ls.scopes, &scope{
		locals: []string{},
		rcLike: map[string]bool{},
		moved:  map[string]bool{},
	})
}
func (ls *lowerState) pop() *scope {
	last := ls.scopes[len(ls.scopes)-1]
	ls.scopes = ls.scopes[:len(ls.scopes)-1]
	return last
}
func (ls *lowerState) cur() *scope { return ls.scopes[len(ls.scopes)-1] }

// ---- lowering ----

func (ls *lowerState) lowerBlock(blk *ast.Block) {
	for _, st := range blk.Stmts {
		if ls.terminated {
			break
		}
		ls.lowerStmt(st)
	}
	// End-of-root-block finalization (only for outermost scope and not after return).
	if len(ls.scopes) == 1 && !ls.terminated {
		ls.emitScopeDrops(ls.cur())
	}
}

func (ls *lowerState) lowerStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.LetStmt:
		// shadowing in same scope drops previous
		if ls.hasLocal(s.Name.Name) {
			ls.dropLocalByName(s.Name.Name)
			ls.removeLocal(s.Name.Name)
		}
		// record type shape
		if ls.info != nil {
			if sym := ls.info.Idents[&s.Name]; sym != nil {
				if isRcLike(sym.Type) {
					ls.cur().rcLike[s.Name.Name] = true
				}
			}
		}
		ls.cur().locals = append(ls.cur().locals, s.Name.Name)
		var init hir.Value
		if s.Value != nil {
			// detect trivial move: let y = x
			if id, ok := s.Value.(*ast.Ident); ok {
				init = ls.lowerExpr(s.Value)
				if ls.hasLocal(id.Name) {
					ls.cur().moved[id.Name] = true
				}
			} else {
				init = ls.lowerExpr(s.Value)
			}
		}
		ls.b.Emit(&hir.Let{Name: s.Name.Name, Init: init})

	case *ast.AssignStmt:
		if len(s.LHS) == 1 && len(s.RHS) == 1 {
			lhs := ls.lowerLValue(s.LHS[0])
			rhs := ls.lowerExpr(s.RHS[0])
			ls.b.Emit(&hir.Assign{LHS: lhs, RHS: rhs})
		}

	case *ast.ExprStmt:
		_ = ls.lowerExpr(s.Expr) // materialize side effects if needed

	case *ast.ReturnStmt:
		// Before returning, run defers and drop locals from all open scopes (inner→outer).
		ls.emitAllDefersAndDrops()
		var v hir.Value
		if s.Value != nil {
			v = ls.lowerExpr(s.Value)
		}
		ls.b.Emit(&hir.Ret{Val: v})
		ls.terminated = true

	case *ast.IfStmt:
		cond := ls.lowerExpr(s.Cond)

		thenBlk := ls.b.NewBlock("then")
		oldCur := ls.b.Block()
		ls.push()
		ls.b.SetBlock(thenBlk)
		ls.lowerBlock(s.Then)
		scThen := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(scThen)
		}
		ls.b.SetBlock(oldCur)

		var elseBlk *hir.Block
		if s.Else != nil {
			elseBlk = ls.b.NewBlock("else")
			ls.push()
			ls.b.SetBlock(elseBlk)
			ls.lowerBlock(s.Else)
			scElse := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scElse)
			}
			ls.b.SetBlock(oldCur)
		}
		ls.b.Emit(&hir.If{Cond: cond, Then: thenBlk, Else: elseBlk})

	case *ast.WhileStmt:
		cond := ls.lowerExpr(s.Cond)
		bodyBlk := ls.b.NewBlock("while")
		oldCur := ls.b.Block()
		ls.push()
		ls.b.SetBlock(bodyBlk)
		ls.lowerBlock(s.Body)
		scWhile := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(scWhile)
		}
		ls.b.SetBlock(oldCur)
		ls.b.Emit(&hir.While{Cond: cond, Body: bodyBlk})

	case *ast.UsingStmt:
		// using X [= init]: body
		ls.push()
		ident := ls.nameOf(s.Bind)
		if ident != "" {
			// Bind the handle but do NOT register it as a "local" for scope-drops.
			if s.Init != nil {
				init := ls.lowerExpr(s.Init)
				ls.b.Emit(&hir.Let{Name: ident, Init: init})
			} else {
				ls.b.Emit(&hir.Let{Name: ident})
			}
			// Single destroy at scope end (or return) via defer.
			ls.cur().defers = append(ls.cur().defers, hir.Var{Name: ident})
		}
		ls.lowerBlock(s.Body)
		sc := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(sc)
		}

	case *ast.DeferStmt:
		// For M7A/B, capture a generic "defer drop <target>" if shape is simple.
		if ce := s.Call; ce != nil {
			// if call is "__close(x)" treat as drop x
			if id, ok := ce.Callee.(*ast.Ident); ok && id.Name == "__close" && len(ce.Args) == 1 {
				if v := ls.valueOf(ce.Args[0]); v != nil {
					ls.cur().defers = append(ls.cur().defers, v)
				}
			}
		}
	default:
		// other statements ignored for this phase
	}
}

func (ls *lowerState) lowerExpr(e ast.Expr) hir.Value {
	switch e := e.(type) {
	case *ast.IntLit:
		return hir.ConstInt{Text: e.Text}
	case *ast.BoolLit:
		return hir.ConstBool{Value: e.Value}
	case *ast.StrLit:
		return hir.ConstStr{Text: ""}
	case *ast.Ident:
		return hir.Var{Name: e.Name}
	case *ast.CallExpr:
		fnName := ls.calleeName(e.Callee)
		args := make([]hir.Value, 0, len(e.Args))
		for _, a := range e.Args {
			args = append(args, ls.lowerExpr(a))
		}
		dst := ls.b.FreshTemp("t")
		ls.b.Emit(&hir.Call{Fn: fnName, Args: args, Dst: dst})
		return dst
	case *ast.FieldExpr:
		return hir.Var{Name: ls.fieldName(e)}
	default:
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

func (ls *lowerState) calleeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.FieldExpr:
		return ls.fieldName(x)
	default:
		var buf bytes.Buffer
		ast.Print(&buf, x)
		return buf.String()
	}
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

// emit drops for one scope (locals in reverse declaration order, then defers LIFO)
func (ls *lowerState) emitScopeDrops(sc *scope) {
	// locals
	for i := len(sc.locals) - 1; i >= 0; i-- {
		name := sc.locals[i]
		if sc.moved[name] {
			continue // skip moved-from owner
		}
		if sc.rcLike[name] {
			ls.b.Emit(&hir.DecRef{Val: hir.Var{Name: name}})
		} else {
			ls.b.Emit(&hir.Drop{Val: hir.Var{Name: name}})
		}
	}
	// defers (modeled as drops of values)
	for i := len(sc.defers) - 1; i >= 0; i-- {
		v := sc.defers[i]
		// If the deferred target is an rc-like variable, decref; otherwise drop.
		if varName, ok := v.(hir.Var); ok && sc.rcLike[varName.Name] {
			ls.b.Emit(&hir.DecRef{Val: v})
		} else {
			ls.b.Emit(&hir.Drop{Val: v})
		}
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
			delete(sc.rcLike, name)
			delete(sc.moved, name)
			return
		}
	}
}

// dropLocalByName emits an immediate drop/decref for a local (used on shadowing).
func (ls *lowerState) dropLocalByName(name string) {
	sc := ls.cur()
	if sc.rcLike[name] {
		ls.b.Emit(&hir.DecRef{Val: hir.Var{Name: name}})
	} else {
		ls.b.Emit(&hir.Drop{Val: hir.Var{Name: name}})
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

// --- type predicates

func isRcLike(t types.T) bool {
	switch t.(type) {
	case *types.Rc, *types.Arc:
		return true
	default:
		return false
	}
}
