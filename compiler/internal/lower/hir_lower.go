package lower

import (
	"bytes"
	"fmt"
	"unicode/utf8"

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
		b:                   b,
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}}}, // root
		terminated:          false,
		info:                info,
		src:                 nil,
		tempsFromArenaAlloc: map[string]bool{},
	}
	ls.lowerBlock(blk)
	return b.Func()
}

// LowerBlockFromSource behaves like LowerBlock but can materialize string literals
// by scanning the original source using (line,col) from StrLit.Span.
func LowerBlockFromSource(name string, blk *ast.Block, src []byte) *hir.Func {
	b := hir.NewFunc(name)
	ls := &lowerState{
		b:                   b,
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}}}, // root
		terminated:          false,
		info:                nil,
		src:                 src,
		tempsFromArenaAlloc: map[string]bool{},
	}
	ls.lowerBlock(blk)
	return b.Func()
}

type lowerState struct {
	b          *hir.Builder
	scopes     []*scope // stack
	terminated bool     // set once a return is emitted
	info       *check.Info
	src        []byte // optional: original source for literal materialization

	tempsFromArenaAlloc map[string]bool // temp.Name -> true if produced by ArenaAlloc
}

type scope struct {
	locals     []string        // in declaration order
	rcLike     map[string]bool // locals that are rc/arc
	moved      map[string]bool // locals moved-from; skip drop
	defers     []hir.Value
	arenas     map[string]bool // names that are arena handles in this scope
	arenaOwned map[string]bool // locals whose storage originates from arena.alloc
}

func (ls *lowerState) push() {
	ls.scopes = append(ls.scopes, &scope{
		locals:     []string{},
		rcLike:     map[string]bool{},
		moved:      map[string]bool{},
		defers:     []hir.Value{},
		arenas:     map[string]bool{},
		arenaOwned: map[string]bool{},
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
	// End-of-root-block finalization (only for outermost scope).
	if len(ls.scopes) == 1 && !ls.terminated {
		ls.emitScopeDrops(ls.cur())
		// If no explicit return ran, emit a default one (Tier-0: i32 0).
		ls.b.Emit(&hir.Ret{Val: nil})
		ls.terminated = true
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

		// M7C: if init was a temp that came from ArenaAlloc, mark this local as arena-owned.
		if t, ok := init.(hir.Temp); ok && ls.tempsFromArenaAlloc[t.Name] {
			ls.cur().arenaOwned[s.Name.Name] = true
		}

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
		// using X [= init]: body  → bind handle + defer destroy_arena(X)
		ls.push()
		ident := ls.nameOf(s.Bind)
		if ident != "" {
			// Don't register as "local" (avoid ordinary drop); we destroy via defer.
			if s.Init != nil {
				init := ls.lowerExpr(s.Init)
				ls.b.Emit(&hir.Let{Name: ident, Init: init})
			} else {
				ls.b.Emit(&hir.Let{Name: ident})
			}
			// Mark this name as an arena handle in the current scope.
			ls.cur().arenas[ident] = true
			// One destroy at scope end (or return) via defer.
			ls.cur().defers = append(ls.cur().defers, hir.Var{Name: ident})
		}
		ls.lowerBlock(s.Body)
		sc := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(sc)
		}

	case *ast.DeferStmt:
		// Keep the basic "__close(x)" → drop/decref path from M7A/B.
		if ce := s.Call; ce != nil {
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

// Map (1-based line,col) to byte index; returns -1 if out-of-range.
func byteOffsetFromLineCol(src []byte, line, col int) int {
	if line < 1 || col < 1 {
		return -1
	}
	ln := 1
	i := 0
	// advance to the target line
	for i < len(src) && ln < line {
		if src[i] == '\n' {
			ln++
		}
		i++
	}
	if ln != line {
		return -1
	}
	// now at start of the target line; advance col-1 runes
	c := 1
	for i < len(src) && src[i] != '\n' && c < col {
		_, w := utf8.DecodeRune(src[i:])
		if w == 0 {
			return -1
		}
		i += w
		c++
	}
	if c != col {
		return -1
	}
	return i
}

// scanStringLiteral expects src at the opening quote (or opening """ if long)
// and returns the raw (unescaped) contents between quotes.
// It tolerates backslash-escaped quotes by skipping the backslash.
func scanStringLiteral(src []byte, line, col int, long bool) (string, bool) {
	i := byteOffsetFromLineCol(src, line, col)
	if i < 0 || i >= len(src) {
		return "", false
	}
	if long {
		// expect """
		if i+3 > len(src) || !(src[i] == '"' && src[i+1] == '"' && src[i+2] == '"') {
			return "", false
		}
		i += 3
		start := i
		for i < len(src) {
			// close only on exact """
			if i+3 <= len(src) && src[i] == '"' && src[i+1] == '"' && src[i+2] == '"' {
				return string(src[start:i]), true
			}
			// skip escapes minimally
			if src[i] == '\\' && i+1 < len(src) {
				i += 2
				continue
			}
			_, w := utf8.DecodeRune(src[i:])
			if w == 0 {
				break
			}
			i += w
		}
		return "", false
	}

	// short string: expect "
	if src[i] != '"' {
		return "", false
	}
	i++ // after opening "
	start := i
	for i < len(src) {
		// closing quote not escaped
		if src[i] == '"' {
			return string(src[start:i]), true
		}
		// handle simple escapes so we don't wrongly stop at \".
		if src[i] == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if src[i] == '\n' {
			break
		}
		_, w := utf8.DecodeRune(src[i:])
		if w == 0 {
			break
		}
		i += w
	}
	return "", false
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
		if sc.arenaOwned[name] {
			continue // arena-backed values are freed by destroy_arena only
		}
		if sc.rcLike[name] {
			ls.b.Emit(&hir.DecRef{Val: hir.Var{Name: name}})
		} else {
			ls.b.Emit(&hir.Drop{Val: hir.Var{Name: name}})
		}
	}
	// defers
	for i := len(sc.defers) - 1; i >= 0; i-- {
		v := sc.defers[i]
		// Arena handle?
		if varName, ok := v.(hir.Var); ok && sc.arenas[varName.Name] {
			ls.b.Emit(&hir.DestroyArena{Arena: v})
			continue
		}
		// rc-like target?
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

func (ls *lowerState) hasArena(name string) bool {
	for i := len(ls.scopes) - 1; i >= 0; i-- {
		if ls.scopes[i].arenas[name] {
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
			delete(sc.arenaOwned, name)
			return
		}
	}
}

// dropLocalByName emits an immediate drop/decref for a local (used on shadowing).
func (ls *lowerState) dropLocalByName(name string) {
	sc := ls.cur()
	if sc.arenaOwned[name] {
		// arena-backed locals are not individually dropped
		return
	}
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

// lowerExpr converts surface AST expressions to HIR values (Tier-0).
// NOTE: This stays intentionally minimal: enough for async demos, arena alloc,
//
//	prelude stubs, and (now) list comprehensions ➜ list_push calls.
func (ls *lowerState) lowerExpr(e ast.Expr) hir.Value {
	switch x := e.(type) {
	case *ast.IntLit:
		return &hir.ConstInt{Text: x.Text}
	case *ast.BoolLit:
		return &hir.ConstBool{Value: x.Value}
	case *ast.StrLit:
		// String literal payload is handled elsewhere; here we just mark "str".
		return &hir.ConstStr{Text: "<lit>"}
	case *ast.Ident:
		return hir.Var{Name: x.Name}

	case *ast.UnaryExpr:
		// Await (async) is the only unary we lower in Tier-0.
		if x.Op == "await" {
			// await <expr>
			dst := ls.b.FreshTemp("await")
			fut := ls.lowerExpr(x.X)
			ls.b.Emit(&hir.Await{Dst: dst, Fut: fut})
			return dst
		}
		// Unknown unary: just print-through for now.
		return hir.Var{Name: fmt.Sprintf("unary(%s …)", x.Op)}

	case *ast.CallExpr:
		// Determine a printable callee name for Ident or FieldExpr.
		callee := ls.calleeName(x.Callee)

		// Lower arguments first.
		var args []hir.Value
		for _, a := range x.Args {
			args = append(args, ls.lowerExpr(a))
		}

		// Special-cases for arena helpers (support Ident("arena.alloc") or FieldExpr arena.alloc).
		switch callee {
		case "arena.alloc":
			dst := ls.b.FreshTemp("alloc")
			ls.b.Emit(&hir.Call{Dst: dst, Fn: "arena.alloc", Args: args})
			// Mark temp as coming from arena.alloc so let-binding flags arenaOwned.
			ls.tempsFromArenaAlloc[dst.Name] = true
			return dst
		case "arena.register_poll":
			ls.b.Emit(&hir.Call{Fn: "arena.register_poll", Args: args})
			return nil
		}

		// Generic call: emit `%t = call <callee>(args...)` if value needed.
		dst := ls.b.FreshTemp("call")
		if callee == "" {
			callee = "<call>"
		}
		ls.b.Emit(&hir.Call{Dst: dst, Fn: callee, Args: args})
		return dst

	case *ast.FieldExpr:
		// For expressions used as values, represent field access via a helper call.
		base := ls.lowerExpr(x.X)
		name := x.Name.Name
		dst := ls.b.FreshTemp("field")
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "get.field." + name, Args: []hir.Value{base}})
		return dst

	case *ast.ListComp:
		return ls.lowerListComp(x)

	default:
		// Print-through placeholder for anything not wired yet.
		return hir.Var{Name: fmt.Sprintf("<expr:%T>", e)}
	}
}

// lowerListComp builds a tiny HIR shape for list comprehensions:
//
//	let %res
//	call list_push(%res, <elem>)
//
// Returns %res as the value of the comprehension.
//
// Tier-0 note: This is a compile-only skeleton. We don't yet expand
// the full generator chain; instead, we ensure the result handle exists
// and we append the element once. Later passes can elaborate to real loops.
func (ls *lowerState) lowerListComp(c *ast.ListComp) hir.Value {
	// Result handle
	res := ls.b.FreshTemp("list")
	ls.b.Emit(&hir.Let{Name: res.Name})

	// Element value
	elem := ls.lowerExpr(c.Elem)
	if elem == nil {
		// Be defensive; use a const 0 if lowering produced nothing.
		elem = &hir.ConstInt{Text: "0"}
	}

	// For now: a single append with prelude stub. (Tight loop elab comes next.)
	ls.b.Emit(&hir.Call{Fn: "list_push", Args: []hir.Value{res, elem}})

	return res
}
