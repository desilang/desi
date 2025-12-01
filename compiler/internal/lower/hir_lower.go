package lower

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
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
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}, mutable: map[string]bool{}, types: map[string]types.T{}, tempDrops: map[string]bool{}}}, // root
		terminated:          false,
		info:                info,
		src:                 nil,
		tempsFromArenaAlloc: map[string]bool{},
		matchLocals:         map[string]hir.Value{},
	}
	ls.lowerBlock(blk)
	return b.Func()
}

// LowerBlockFromSource behaves like LowerBlock but can materialize string literals
// by scanning the original source using (line,col) from StrLit.Span.
// LowerFuncFromDecl lowers a function declaration to HIR, including its parameters.
func LowerFuncFromDecl(fd *ast.FuncDecl, info *check.Info, src []byte) *hir.Func {
	b := hir.NewFunc(fd.Name.Name)
	ls := &lowerState{
		b:                   b,
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}, mutable: map[string]bool{}, types: map[string]types.T{}, tempDrops: map[string]bool{}}}, // root
		terminated:          false,
		info:                info,
		src:                 src,
		tempsFromArenaAlloc: map[string]bool{},
		matchLocals:         map[string]hir.Value{},
	}
	ls.lowerBlock(fd.Body)
	f := b.Func()
	f.Origin = fd // Track AST origin for move analysis lookup

	// Populate parameters from AST
	// For variadic functions, the last parameter has already been wrapped in list[T] by the type checker
	// We need to get the types from the type checker's info
	if info != nil {
		// Try to get the function type from info.Funcs
		if set, ok := info.Funcs[fd.Name.Name]; ok && len(set.Cands) > 0 {
			// Use the first candidate (should be the only one for this function)
			funcType := set.Cands[0].Type
			if funcType != nil {
				// M15: For generic functions, use erased signatures (all ptr)
				isGeneric := len(funcType.TypeParams) > 0 || len(fd.TypeParams) > 0

				if isGeneric {
					// Generic function: all parameters and return type are ptr
					for _, p := range fd.Params {
						f.Params = append(f.Params, hir.Param{Name: p.Name.Name, Type: "ptr"})
					}
					// Return type is also erased to ptr (unless it's void/none)
					if funcType.Ret != nil && !types.Equal(funcType.Ret, types.None) {
						f.RetType = "ptr"
					}
				} else {
					// Non-generic function: use actual types
					// Set return type
					if funcType.Ret != nil {
						retType := lowerType(funcType.Ret)
						// Don't set void - let backend use its defaults (e.g. i32 for main)
						if retType != "void" {
							f.RetType = retType
						}
					}

					// Set parameter types
					for i, p := range fd.Params {
						paramType := "ptr" // default
						if i < len(funcType.Params) && funcType.Params[i] != nil {
							paramType = lowerType(funcType.Params[i])
						}
						f.Params = append(f.Params, hir.Param{Name: p.Name.Name, Type: paramType})
					}
				}
				return f
			}
		}
	}

	// Fallback: use default types
	for _, p := range fd.Params {
		f.Params = append(f.Params, hir.Param{Name: p.Name.Name, Type: "ptr"})
	}
	if fd.RetType != nil {
		switch fd.RetType.Name {
		case "str":
			f.RetType = "ptr"
		case "int":
			f.RetType = "i32"
		case "bool":
			f.RetType = "i1"
		case "float":
			f.RetType = "double"
		case "none":
			f.RetType = "void"
		default:
			f.RetType = "ptr"
		}
	}

	return f
}

// LowerBlockFromSource lowers just a block with a given name (for compatibility with existing code).
func LowerBlockFromSource(name string, blk *ast.Block, info *check.Info, src []byte) *hir.Func {
	b := hir.NewFunc(name)
	ls := &lowerState{
		b:                   b,
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}, mutable: map[string]bool{}, types: map[string]types.T{}, tempDrops: map[string]bool{}}}, // root
		terminated:          false,
		info:                info,
		src:                 src,
		tempsFromArenaAlloc: map[string]bool{},
		matchLocals:         map[string]hir.Value{},
	}
	ls.lowerBlock(blk)
	return b.Func()
}

// LowerDefaultToStr generates a default to_str implementation that returns the type name.
func LowerDefaultToStr(name, typeName string) *hir.Func {
	b := hir.NewFunc(name)
	f := b.Func()
	f.Params = []hir.Param{{Name: "self", Type: "ptr"}}
	f.RetType = "ptr"
	// For M14, just return the type name as a string.
	// TODO: Generate "TypeName(field=val, ...)"
	b.Emit(&hir.Ret{Val: hir.ConstStr{Text: typeName}})
	return f
}

type lowerState struct {
	b          *hir.Builder
	scopes     []*scope // stack
	terminated bool     // set once a return is emitted
	info       *check.Info
	src        []byte // optional: original source for literal materialization

	tempsFromArenaAlloc map[string]bool      // temp.Name -> true if produced by ArenaAlloc
	matchLocals         map[string]hir.Value // pattern binding variables (name -> HIR value)
}

type scope struct {
	locals     []string        // in declaration order
	rcLike     map[string]bool // locals that are rc/arc
	moved      map[string]bool // locals moved-from; skip drop
	defers     []hir.Value
	arenas     map[string]bool    // names that are arena handles in this scope
	arenaOwned map[string]bool    // locals whose storage originates from arena.alloc
	mutable    map[string]bool    // variables declared with mut
	types      map[string]types.T // variable types for Drop
	tempDrops  map[string]bool    // temporaries that need to be dropped
}

func (ls *lowerState) push() {
	ls.scopes = append(ls.scopes, &scope{
		locals:     []string{},
		rcLike:     map[string]bool{},
		moved:      map[string]bool{},
		tempDrops:  map[string]bool{},
		defers:     []hir.Value{},
		arenas:     map[string]bool{},
		arenaOwned: map[string]bool{},
		mutable:    map[string]bool{},
		types:      map[string]types.T{},
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
		// record type shape and extract type from type checker
		var varType interface{}
		if ls.info != nil {
			if sym := ls.info.Idents[&s.Name]; sym != nil {
				varType = sym.Type
				if isRcLike(sym.Type) {
					ls.cur().rcLike[s.Name.Name] = true
				}
			}
		}
		ls.cur().locals = append(ls.cur().locals, s.Name.Name)

		// Store type for Drop instruction
		// IMPORTANT: Don't track variables initialized from index expressions for drop
		// Index expressions return borrowed references, not owned values
		skipDrop := false
		if s.Value != nil {
			if _, isIndexExpr := s.Value.(*ast.IndexExpr); isIndexExpr {
				skipDrop = true
			}
		}

		if varType != nil && !skipDrop {
			if t, ok := varType.(types.T); ok {
				ls.cur().types[s.Name.Name] = t
			}
		}

		// Track if variable is mutable
		if s.Mutable {
			ls.cur().mutable[s.Name.Name] = true
		}

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

		// For mutable variables, always allocate storage
		if s.Mutable {
			// Emit Let without init (will be allocated by backend)
			ls.b.Emit(&hir.Let{Name: s.Name.Name, Init: nil, Type: varType})
			// If there's an initial value, store it
			if init != nil {
				ls.b.Emit(&hir.Store{
					Dst: hir.Var{Name: s.Name.Name},
					Val: init,
				})
			}
		} else {
			// Immutable: use SSA directly
			ls.b.Emit(&hir.Let{Name: s.Name.Name, Init: init, Type: varType})
		}

		// M7C: if init was a temp that came from ArenaAlloc, mark this local as arena-owned.
		if t, ok := init.(hir.Temp); ok {
			if ls.tempsFromArenaAlloc[t.Name] {
				ls.cur().arenaOwned[s.Name.Name] = true
			}
			// Consume temp so it's not dropped
			ls.consumeTemp(t)
		}

	case *ast.AssignStmt:
		if len(s.LHS) == 1 && len(s.RHS) == 1 {
			lhs := ls.lowerLValue(s.LHS[0])
			rhs := ls.lowerExpr(s.RHS[0])

			// Check if this is a mutable variable assignment
			isMutable := false
			if lhsName, ok := s.LHS[0].(*ast.Ident); ok {
				if ls.isMutable(lhsName.Name) {
					isMutable = true
				}
			}

			if isMutable {
				// Emit Store for mutable variables
				ls.b.Emit(&hir.Store{
					Dst: hir.Var{Name: lhs},
					Val: rhs,
				})
				ls.consumeTemp(rhs) // Store consumes the value
			} else {
				// Emit Assign for SSA-style variables
				ls.b.Emit(&hir.Assign{LHS: lhs, RHS: rhs})
				ls.consumeTemp(rhs) // Assign consumes the value
			}
		}

	case *ast.ExprStmt:
		_ = ls.lowerExpr(s.Expr) // materialize side effects if needed

	case *ast.MatchExpr:
		_ = ls.lowerMatchExpr(s)

	case *ast.ReturnStmt:
		// Before returning, run defers and drop locals from all open scopes (inner→outer).
		ls.emitAllDefersAndDrops()
		var v hir.Value
		if s.Value != nil {
			v = ls.lowerExpr(s.Value)
			ls.consumeTemp(v) // Return consumes the value
		}
		ls.b.Emit(&hir.Ret{Val: v})
		ls.terminated = true

	case *ast.IfStmt:
		cond := ls.lowerExpr(s.Cond)

		thenBlk := ls.b.NewBlock("then")
		oldCur := ls.b.Block()

		// Save terminated state
		wasTerminated := ls.terminated
		ls.terminated = false // Start fresh for the block

		ls.push()
		ls.b.SetBlock(thenBlk)
		ls.lowerBlock(s.Then)
		scThen := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(scThen)
		}
		thenTerminated := ls.terminated
		ls.b.SetBlock(oldCur)

		var elseBlk *hir.Block
		elseTerminated := false
		if s.Else != nil {
			elseBlk = ls.b.NewBlock("else")

			ls.terminated = false // Start fresh for the block

			ls.push()
			ls.b.SetBlock(elseBlk)
			ls.lowerBlock(s.Else)
			scElse := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scElse)
			}
			elseTerminated = ls.terminated
			ls.b.SetBlock(oldCur)

			// If both branches terminate, the if statement terminates
			ls.terminated = wasTerminated || (thenTerminated && elseTerminated)
		} else {
			// If no else, execution continues (unless already terminated before)
			ls.terminated = wasTerminated
		}

		ls.b.Emit(&hir.If{Cond: cond, Then: thenBlk, Else: elseBlk})

	case *ast.WhileStmt:
		// Create a condition block that will be re-evaluated each iteration
		condBlk := ls.b.NewBlock("while_cond")
		oldCur := ls.b.Block()

		// Lower condition in the condition block
		ls.b.SetBlock(condBlk)
		cond := ls.lowerExpr(s.Cond)
		ls.b.SetBlock(oldCur)

		// Create body block
		bodyBlk := ls.b.NewBlock("while_body")
		ls.push()
		ls.b.SetBlock(bodyBlk)
		ls.lowerBlock(s.Body)
		scWhile := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(scWhile)
		}
		ls.b.SetBlock(oldCur)

		// Emit While with both condition block and body block
		ls.b.Emit(&hir.While{Cond: cond, CondBlock: condBlk, Body: bodyBlk})

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

// lowerExpr is not fully provided in the context, but the edit implies its structure.
// Assuming it looks something like this:
/*
func (ls *lowerState) lowerExpr(e ast.Expr) hir.Value {
	switch x := e.(type) {
	// ... other cases ...
	case *ast.MatchExpr:
		return ls.lowerMatchExpr(x)
	default:
		// ... default handling ...
	}
	return nil
}
*/
// The edit is placed to insert the MatchExpr case into such a switch.

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
			ls.b.Emit(&hir.Drop{Val: hir.Var{Name: name}, Type: sc.types[name]})
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
		} else if varName, ok := v.(hir.Var); ok {
			ls.b.Emit(&hir.Drop{Val: v, Type: sc.types[varName.Name]})
		} else {
			ls.b.Emit(&hir.Drop{Val: v})
		}
	}
	// temporaries
	ls.emitTempDrops()
}

func (ls *lowerState) emitAllDefersAndDrops() {
	// Emit all pending defers and locals from inner-most to outer-most.
	for i := len(ls.scopes) - 1; i >= 0; i-- {
		ls.emitScopeDrops(ls.scopes[i])
	}
}

func (ls *lowerState) hasLocal(name string) bool {
	for _, s := range ls.scopes {
		for _, n := range s.locals {
			if n == name {
				return true
			}
		}
	}
	return false
}

func (ls *lowerState) isMutable(name string) bool {
	for _, s := range ls.scopes {
		if s.mutable[name] {
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
		return hir.ConstInt{Text: x.Text}
	case *ast.FloatLit:
		return hir.ConstFloat{Text: x.Text}
	case *ast.BoolLit:
		return hir.ConstBool{Value: x.Value}

	case *ast.NoneLit:
		// Lower none as null pointer (0)
		return &hir.ConstInt{Text: "0"}

	case *ast.StrLit:
		// If Value is populated (F-string part), use it
		if x.Value != "" {
			// Unescape the string to process escape sequences like \n, \t, etc.
			return hir.ConstStr{Text: unescapeString(x.Value)}
		}
		// Otherwise, extract from source
		if ls.src != nil {
			text, ok := scanStringLiteral(ls.src, x.Span.Start.Line, x.Span.Start.Col, x.Long)
			if ok {
				// Unescape the extracted string as well
				return hir.ConstStr{Text: unescapeString(text)}
			}
		}
		// Fallback to placeholder if source unavailable
		return hir.ConstStr{Text: "<lit>"}
	case *ast.FString:
		// F-string: use asprintf for formatting
		// 1. Allocate buffer for result
		bufPtr := ls.b.FreshTemp("fstr_buf")
		ls.b.Emit(&hir.Alloca{Type: "ptr", Dst: bufPtr})

		// 2. Build format string and arguments
		var fmtBuilder strings.Builder
		var args []hir.Value
		args = append(args, bufPtr) // First arg is &bufPtr

		for _, part := range x.Parts {
			switch p := part.(type) {
			case *ast.StrLit:
				// For F-string parts, use the Value field directly
				if p.Value != "" {
					// First unescape escape sequences like \n, \t, etc.
					text := unescapeString(p.Value)
					// Then unescape {{ and }}
					unescaped := strings.ReplaceAll(text, "{{", "{")
					unescaped = strings.ReplaceAll(unescaped, "}}", "}")
					// Escape % for printf
					escaped := strings.ReplaceAll(unescaped, "%", "%%")
					fmtBuilder.WriteString(escaped)
				}
			default:
				// Expression - lower it and add format specifier
				val := ls.lowerExpr(p)
				if ls.info != nil {
					typ := ls.info.Types[p]
					if types.Equal(typ, types.Int) {
						fmtBuilder.WriteString("%lld")
					} else if types.Equal(typ, types.Str) {
						fmtBuilder.WriteString("%s")
					} else if types.Equal(typ, types.Float) {
						fmtBuilder.WriteString("%f")
					} else if types.Equal(typ, types.Bool) {
						fmtBuilder.WriteString("%s")
						// Convert bool to string pointer using runtime helper
						// We need to emit a call: bool_to_cstring(val) -> ptr
						res := ls.b.FreshTemp("bool_str")
						ls.b.Emit(&hir.Call{Dst: res, Fn: "bool_to_cstring", Args: []hir.Value{val}})
						val = res
					} else {
						fmtBuilder.WriteString("<?>")
					}
				} else {
					fmtBuilder.WriteString("%s")
				}
				args = append(args, val)
			}
		}

		// 3. Create format string constant and build final args
		fmtStr := hir.ConstStr{Text: fmtBuilder.String()}
		finalArgs := make([]hir.Value, 0, len(args)+1)
		finalArgs = append(finalArgs, args[0])     // bufPtr
		finalArgs = append(finalArgs, fmtStr)      // format string
		finalArgs = append(finalArgs, args[1:]...) // remaining args

		// 4. Call asprintf
		ls.b.Emit(&hir.Call{Fn: "asprintf", Args: finalArgs})

		// 5. Load result from buffer
		res := ls.b.FreshTemp("fstr_res")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: bufPtr, Dst: res})

		return res
	case *ast.TupleLit:
		// Tuples are heap-allocated using arena allocator to support returning from generic functions.
		// For Tier-0 Generics (Type Erasure), ALL tuple elements are boxed to 'ptr'.
		// This ensures layout compatibility between (T, T) -> {ptr, ptr} and (int, int).
		var elemTypes []string
		for range x.Elems {
			elemTypes = append(elemTypes, "ptr")
		}
		structType := "{" + strings.Join(elemTypes, ", ") + "}"

		// Calculate struct size (ptr = 8 bytes on 64-bit, so N elements = N * 8)
		structSize := len(x.Elems) * 8

		// Allocate on heap using malloc
		dst := ls.b.FreshTemp("tuple_ptr")
		sizeVal := hir.ConstInt{Text: fmt.Sprintf("%d", structSize), Type: "i64"}
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "malloc", Args: []hir.Value{sizeVal}, Type: "ptr"})

		// Store elements
		for i, e := range x.Elems {
			val := ls.lowerExpr(e)

			// Box if necessary (allocate + store for primitives)
			valType := "ptr" // default
			if ls.info != nil {
				if t := ls.info.Types[e]; t != nil {
					valType = lowerType(t)
				}
			}

			var boxedVal hir.Value = val
			if valType != "ptr" && valType != "void" {
				// Allocate storage for the value on heap
				boxPtr := ls.b.FreshTemp("elem_box_ptr")
				elemSize := hir.ConstInt{Text: "8", Type: "i64"} // conservative: always 8 bytes
				if valType == "i32" || valType == "i1" {
					elemSize = hir.ConstInt{Text: "4", Type: "i64"}
				}
				ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{elemSize}, Type: "ptr"})
				// Store the value
				ls.b.Emit(&hir.Store{Dst: boxPtr, Val: val})
				boxedVal = boxPtr
			}

			// GEP
			fieldPtr := ls.b.FreshTemp("tuple_field")
			ls.b.Emit(&hir.GetElementPtr{
				Type:    structType,
				Base:    dst,
				Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
				Dst:     fieldPtr,
			})

			// Store
			ls.b.Emit(&hir.Store{Dst: fieldPtr, Val: boxedVal})
		}
		return dst

	case *ast.SliceExpr:
		// list[start:end] -> list_slice(list, start, end)
		list := ls.lowerExpr(x.X)

		// Start index (default 0)
		var start hir.Value
		if x.I != nil {
			val := ls.lowerExpr(x.I)
			// Cast i32 to i64 for runtime call
			start = ls.b.FreshTemp("start_i64")
			ls.b.Emit(&hir.Cast{Dst: start.(hir.Temp), Src: val, Type: "i64"})
		} else {
			start = hir.ConstInt{Text: "0", Type: "i64"}
		}

		// End index (default len(list))
		var end hir.Value
		if x.J != nil {
			val := ls.lowerExpr(x.J)
			// Cast i32 to i64 for runtime call
			end = ls.b.FreshTemp("end_i64")
			ls.b.Emit(&hir.Cast{Dst: end.(hir.Temp), Src: val, Type: "i64"})
		} else {
			// Call list_len -> i64
			end = ls.b.FreshTemp("len_i64")
			ls.b.Emit(&hir.Call{Dst: end.(hir.Temp), Fn: "list_len", Args: []hir.Value{list}})
		}

		// Call list_slice
		res := ls.b.FreshTemp("slice")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_slice", Args: []hir.Value{list, start, end}})
		return res

	case *ast.IndexExpr:
		lhsType := ls.info.Types[x.X]
		if tup, ok := lhsType.(*types.Tuple); ok {
			base := ls.lowerExpr(x.X) // pointer to tuple
			idxLit, _ := x.Idx.(*ast.IntLit)
			idx, _ := strconv.Atoi(idxLit.Text)

			// Reconstruct struct type string for GEP (all ptrs)
			var elemTypes []string
			for range tup.Elems {
				elemTypes = append(elemTypes, "ptr")
			}
			structType := "{" + strings.Join(elemTypes, ", ") + "}"

			// GEP
			fieldPtr := ls.b.FreshTemp("tuple_elem_ptr")
			ls.b.Emit(&hir.GetElementPtr{
				Type:    structType,
				Base:    base,
				Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", idx)}},
				Dst:     fieldPtr,
			})

			// Load ptr
			dstPtr := ls.b.FreshTemp("tuple_elem_ptr_val")
			ls.b.Emit(&hir.Load{
				Type: "ptr",
				Src:  fieldPtr,
				Dst:  dstPtr,
			})

			// Unbox if necessary (load from pointer for primitives)
			targetType := lowerType(tup.Elems[idx])
			if targetType != "ptr" && targetType != "void" {
				// dstPtr points to allocated storage containing the primitive value
				// Load the actual value
				unboxed := ls.b.FreshTemp("unboxed")
				ls.b.Emit(&hir.Load{
					Type: targetType,
					Src:  dstPtr,
					Dst:  unboxed,
				})
				return unboxed
			}

			return dstPtr
		}

		// Handle list indexing: list[index]
		if listType, ok := lhsType.(*types.List); ok {
			list := ls.lowerExpr(x.X)
			index := ls.lowerExpr(x.Idx)

			// list_get returns void* (ptr)
			ptrResult := ls.b.FreshTemp("elem_ptr")
			ls.b.Emit(&hir.Call{Dst: ptrResult, Fn: "list_get", Args: []hir.Value{list, index}})

			// Unbox if element type is primitive
			// Check if we need to convert ptr -> int/bool/float
			needsUnboxing := false
			targetType := "i32" // default

			if listType.Elem != nil {
				switch listType.Elem.String() {
				case "int":
					needsUnboxing = true
					targetType = "i32"
				case "bool":
					needsUnboxing = true
					targetType = "i1"
				case "float":
					needsUnboxing = true
					targetType = "double"
					// pointers (str, list, dict, etc.) don't need unboxing
				}
			}

			if needsUnboxing {
				// Cast ptr back to primitive type (ptrtoint)
				dst := ls.b.FreshTemp("elem")
				ls.b.Emit(&hir.Cast{Dst: dst, Src: ptrResult, Type: targetType})
				return dst
			}

			// For pointer types, return as-is
			return ptrResult
		}
		return hir.Var{Name: "<index_expr>"}

	case *ast.Ident:
		// Check if this is a pattern binding variable first
		if val, ok := ls.matchLocals[x.Name]; ok {
			return val
		}

		// If this is a mutable variable, emit a Load instruction
		if ls.isMutable(x.Name) {
			dst := ls.b.FreshTemp("load")

			// Determine type
			loadType := "i32" // default
			if ls.info != nil {
				if sym := ls.info.Idents[x]; sym != nil {
					loadType = lowerType(sym.Type)
				}
			}

			ls.b.Emit(&hir.Load{
				Type: loadType,
				Src:  hir.Var{Name: x.Name},
				Dst:  dst,
			})
			return dst
		}
		return hir.Var{Name: x.Name}

	case *ast.UnaryExpr:
		// Await (async) is the only unary we lower in Tier-0.
		if x.Op == "await" {
			// await <expr>
			dst := ls.b.FreshTemp("await")
			fut := ls.lowerExpr(x.X)
			ls.b.Emit(&hir.Await{Dst: dst, Fut: fut})
			return dst
		} else if x.Op == "-" {
			// Unary minus: 0 - x
			val := ls.lowerExpr(x.X)
			dst := ls.b.FreshTemp("neg")

			// Determine type
			typ := "i64" // default
			if ls.info != nil {
				if t := ls.info.Types[x]; t != nil {
					typ = lowerType(t)
				}
			}

			var zero hir.Value
			if typ == "double" || typ == "float" {
				zero = hir.ConstFloat{Text: "0.0"}
				// BinaryOp lowering handles operator mapping, but we might need explicit opcode if we want fsub
				// Actually, BinaryOp lowering maps "-" to "sub". We need to update BinaryOp lowering to handle floats too!
				// For now, let's assume BinaryOp lowering will be fixed to handle floats.
				// Wait, BinaryOp lowering maps "-" to "sub" unconditionally.
			} else {
				zero = hir.ConstInt{Text: "0"}
			}

			ls.b.Emit(&hir.BinaryOp{
				Op:   "-",
				LHS:  zero,
				RHS:  val,
				Dst:  dst,
				Type: typ,
			})
			return dst
		} else if x.Op == "not" || x.Op == "!" {
			// Logical not: x ^ 1 (xor with true)
			val := ls.lowerExpr(x.X)
			dst := ls.b.FreshTemp("not")
			ls.b.Emit(&hir.BinaryOp{
				Op:   "==",
				LHS:  val,
				RHS:  hir.ConstBool{Value: false}, // x == false is equivalent to not x
				Dst:  dst,
				Type: "i1",
			})
			return dst
		}

		// Unknown unary: just print-through for now.
		return hir.Var{Name: fmt.Sprintf("unary(%s …)", x.Op)}

	case *ast.CallExpr:
		return ls.lowerCall(x)

	case *ast.FieldExpr:
		base := ls.lowerExpr(x.X)
		name := x.Name.Name

		// Check if base is a struct (or generic struct)
		var st *types.Struct
		baseType := ls.info.Types[x.X]
		if s, ok := baseType.(*types.Struct); ok {
			st = s
		} else if g, ok := baseType.(*types.Generic); ok {
			if s, ok := g.Base.(*types.Struct); ok {
				st = s
			}
		}

		if st != nil {
			// Find field index and offset
			idx := -1
			offset := 0
			for i, f := range st.Fields {
				if f.Name == name {
					idx = i
					break
				}
				offset += getSize(f.Type)
			}

			if idx != -1 {
				// Emit GEP + Load
				fieldPtr := ls.b.FreshTemp("field_ptr")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    "i8", // struct is i8 array
					Base:    base,
					Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
					Dst:     fieldPtr,
				})

				dst := ls.b.FreshTemp("field_val")
				// Determine field type for Load (storage type)
				storageType := lowerType(st.Fields[idx].Type)

				ls.b.Emit(&hir.Load{
					Type:     storageType,
					Src:      fieldPtr,
					Dst:      dst,
					DesiType: st.Fields[idx].Type,
				})

				// Check if unboxing is needed (Generic T -> primitive)
				// If storage is ptr (from T) but expression type is primitive (e.g. int)
				exprType := ls.info.Types[x]
				targetType := lowerType(exprType)

				if storageType == "ptr" && targetType != "ptr" && targetType != "void" {
					// Unbox: ptr -> targetType
					unboxed := ls.b.FreshTemp("unboxed")
					ls.b.Emit(&hir.Cast{Dst: unboxed, Src: dst, Type: targetType})
					return unboxed
				}

				return dst
			}
		}

		// Fallback for methods (dict/set) or unknown types
		dst := ls.b.FreshTemp("field")
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "get.field." + name, Args: []hir.Value{base}})
		return dst

	case *ast.ListLit:
		// Create new list
		res := ls.b.FreshTemp("list")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_new", Args: []hir.Value{}})

		// Append elements
		for _, e := range x.Elems {
			val := ls.lowerExpr(e)

			// Cast to ptr for generic storage (void*)
			// This handles both pointers (bitcast) and integers (inttoptr)
			valPtr := ls.b.FreshTemp("val_ptr")
			ls.b.Emit(&hir.Cast{Dst: valPtr, Src: val, Type: "ptr"})

			ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{res, valPtr}})
		}
		return res

	case *ast.DictLit:
		return ls.lowerDictLit(x)

	case *ast.SetLit:
		if ls.info != nil {
			if t, ok := ls.info.Types[x].(*types.Set); ok {
				return ls.lowerSetLit(x, t)
			}
		}
		return hir.Var{Name: "<set_lit_error>"}

	case *ast.ListComp:
		return ls.lowerListComp(x)

	case *ast.BinaryExpr:
		// Lower binary expressions: arithmetic (+, -, *, /, %, **) and comparisons (<, >, <=, >=, ==, !=)
		lhs := ls.lowerExpr(x.Lhs)
		rhs := ls.lowerExpr(x.Rhs)

		// Determine result type based on operation
		var resultType string
		if x.Op == "==" || x.Op == "!=" || x.Op == "<" || x.Op == ">" || x.Op == "<=" || x.Op == ">=" {
			// Comparison operations return boolean (i1)
			resultType = "i1"
		} else {
			// Arithmetic operations preserve operand type
			if ls.info != nil {
				if typ := ls.info.Types[x]; typ != nil {
					resultType = lowerType(typ)
				} else {
					resultType = "i32" // default fallback
				}
			} else {
				resultType = "i32" // default fallback
			}
		}

		dst := ls.b.FreshTemp("binop")
		ls.b.Emit(&hir.BinaryOp{
			Op:   x.Op,
			LHS:  lhs,
			RHS:  rhs,
			Dst:  dst,
			Type: resultType,
		})

		// If string concatenation, track result for cleanup
		if x.Op == "+" && resultType == "ptr" {
			// Check if it's actually a string type
			isStr := false
			if ls.info != nil {
				if t, ok := ls.info.Types[x]; ok && types.Equal(t, types.Str) {
					isStr = true
				}
			}
			if isStr {
				ls.addTempDrop(dst.Name)
			}
		}

		return dst

	case *ast.MatchExpr:
		return ls.lowerMatchExpr(x)

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

// lowerDictLit builds HIR for dict literals:
//
//	let %dict = call dict_new(value_size)
//	call dict_insert(%dict, "key1", &val1)
//	call dict_insert(%dict, "key2", &val2)
//	...
//
// Returns %dict as the value of the literal.
//
// Tier-0 note: For now, we assume string keys and pass value size as sizeof(int).
// More sophisticated value handling will come in later tiers.
func (ls *lowerState) lowerDictLit(d *ast.DictLit) hir.Value {
	// Create new dict handle
	// For Tier-0, assume value_size = sizeof(int) = 8 (64-bit)
	res := ls.b.FreshTemp("dict")
	valueSize := &hir.ConstInt{Text: "8"}
	ls.b.Emit(&hir.Call{Dst: res, Fn: "dict_new", Args: []hir.Value{valueSize}})

	// Insert each key-value pair
	for i := range d.Keys {
		key := ls.lowerExpr(d.Keys[i])
		val := ls.lowerExpr(d.Values[i])

		// For Tier-0, we need to pass pointers to the values.
		// Since 'val' might be an immediate (e.g. integer), we spill it to a temp alloca.
		valPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Alloca{Dst: valPtr, Type: "i64", Count: 1})
		ls.b.Emit(&hir.Store{Dst: valPtr, Val: val})

		// Emit a call to dict_insert(dict, key, &value)
		ls.b.Emit(&hir.Call{Fn: "dict_insert", Args: []hir.Value{res, key, valPtr}})
	}

	return res
}

func (ls *lowerState) lowerVariadicCall(x *ast.CallExpr, ft *types.Func) hir.Value {
	callee := ls.calleeName(x.Callee)

	// Fixed params
	nFixed := len(ft.Params) - 1
	var args []hir.Value

	// Lower fixed args (positional only for Tier-0)
	for i := 0; i < nFixed && i < len(x.Args); i++ {
		args = append(args, ls.lowerExpr(x.Args[i]))
	}

	// Excess args
	var excess []hir.Value
	for i := nFixed; i < len(x.Args); i++ {
		excess = append(excess, ls.lowerExpr(x.Args[i]))
	}

	// Construct list
	// List type: ft.Params[nFixed] which is list[T]
	// Elem type T:
	var elemTy string = "ptr" // default
	if lst, ok := ft.Params[nFixed].(*types.List); ok {
		elemTy = lowerType(lst.Elem)
	}

	// 1. Allocate array
	count := len(excess)
	arrDst := ls.b.FreshTemp("varargs_arr")
	if count > 0 {
		ls.b.Emit(&hir.Alloca{Type: elemTy, Count: count, Dst: arrDst})

		// 2. Populate array
		for i, val := range excess {
			// GEP
			ptrDst := ls.b.FreshTemp("elem_ptr")
			idxVal := hir.ConstInt{Text: fmt.Sprintf("%d", i)}
			ls.b.Emit(&hir.GetElementPtr{
				Type:    elemTy,
				Base:    arrDst,
				Indices: []hir.Value{idxVal},
				Dst:     ptrDst,
			})
			// Store
			ls.b.Emit(&hir.Store{Dst: ptrDst, Val: val})
		}
	} else {
		// Allocate 1 dummy element to get a valid pointer
		ls.b.Emit(&hir.Alloca{Type: elemTy, Count: 1, Dst: arrDst})
	}

	// 3. Allocate list struct {ptr, i64}
	listDst := ls.b.FreshTemp("varargs_list")
	ls.b.Emit(&hir.Alloca{Type: "{ptr, i64}", Count: 1, Dst: listDst})

	// 4. Store array ptr to list.0
	// GEP to field 0
	f0Dst := ls.b.FreshTemp("list_ptr")
	ls.b.Emit(&hir.GetElementPtr{
		Type:    "{ptr, i64}",
		Base:    listDst,
		Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: "0"}},
		Dst:     f0Dst,
	})
	ls.b.Emit(&hir.Store{Dst: f0Dst, Val: arrDst})

	// 5. Store len to list.1
	// GEP to field 1 (FIXED: was 0, should be 1)
	f1Dst := ls.b.FreshTemp("list_len")
	ls.b.Emit(&hir.GetElementPtr{
		Type:    "{ptr, i64}",
		Base:    listDst,
		Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: "1"}},
		Dst:     f1Dst,
	})
	// Store as i64
	lenVal := hir.ConstInt{Text: fmt.Sprintf("%d", count), Type: "i64"}
	ls.b.Emit(&hir.Store{Dst: f1Dst, Val: lenVal})

	// Add list to args
	args = append(args, listDst)

	// Emit call
	dst := ls.b.FreshTemp("call")
	ls.b.Emit(&hir.Call{Dst: dst, Fn: callee, Args: args})
	return dst
}

func (ls *lowerState) lowerCall(x *ast.CallExpr) hir.Value {
	// 0. Method calls (FieldExpr callee)
	if fe, ok := x.Callee.(*ast.FieldExpr); ok {
		if ls.info != nil {
			if t, ok := ls.info.Types[fe.X].(*types.Dict); ok {
				return ls.lowerDictMethod(fe, x.Args, t)
			}
			if t, ok := ls.info.Types[fe.X].(*types.Set); ok {
				return ls.lowerSetMethod(fe, x.Args, t)
			}
			if t, ok := ls.info.Types[fe.X].(*types.List); ok {
				return ls.lowerListMethod(fe, x.Args, t)
			}
		}
	}

	// 1. Variadic calls (M14)
	// Look up the function by name to get its signature
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if set, ok := ls.info.Funcs[calleeName]; ok && len(set.Cands) > 0 {
			// Check if any candidate is variadic
			// In practice, after type checking, we know which one was chosen
			// For simplicity, check the first variadic candidate
			// TODO: This could be improved by tracking which candidate was chosen
			for _, cand := range set.Cands {
				if cand.Type != nil && cand.Type.Variadic {
					return ls.lowerVariadicCall(x, cand.Type)
				}
			}
		}
	}

	// 2. M14 Stage 3: print(Display)
	// If we have type info, check if this is print(arg) where arg implements Display.
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if calleeName == "print" && len(x.Args) == 1 {
			// Check if arg implements Display
			// We need the type of the argument.
			// Since we are lowering, we assume check has run.
			// But we don't have easy access to arg type unless we look it up in info.Types.
			// info.Types maps *ast.Expr -> types.T
			if argT := ls.info.Types[x.Args[0]]; argT != nil {
				typeName := argT.String()
				if impls, ok := ls.info.Impls[typeName]; ok {
					if _, hasDisplay := impls["Display"]; hasDisplay {
						// Rewrite to print(TypeName_to_str(arg))
						// 1. Lower arg
						argVal := ls.lowerExpr(x.Args[0])
						// 2. Emit call to to_str
						toStrName := fmt.Sprintf("%s_to_str", typeName)
						strTemp := ls.b.FreshTemp("str")
						ls.b.Emit(&hir.Call{Dst: strTemp, Fn: toStrName, Args: []hir.Value{argVal}})
						// 3. Emit call to print(str)
						dst := ls.b.FreshTemp("print")
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "print", Args: []hir.Value{strTemp}})
						return dst
					}
				}
			}
		}
	}

	// 2. M14 Stage 1: Method Calls (obj.method())
	if fe, ok := x.Callee.(*ast.FieldExpr); ok && ls.info != nil {
		// Check if this is a method call
		// We need the type of the receiver (fe.X)
		if recvT := ls.info.Types[fe.X]; recvT != nil {
			typeName := recvT.String()
			methodName := fe.Name.Name
			// Check if method exists in Impls
			// Note: This is a simplification. We should check if the method was actually resolved to a trait method.
			// But for M14, all methods on structs come from Impls (or are treated similarly).
			if impls, ok := ls.info.Impls[typeName]; ok {
				// Iterate all traits to find the method?
				// Or just check if we can find it.
				// For now, assume if we find it in any trait, it's the one.
				found := false
				for _, methods := range impls {
					for _, m := range methods {
						if m.Name.Name == methodName {
							found = true
							break
						}
					}
					if found {
						break
					}
				}

				if found {
					// Rewrite to TypeName_MethodName(obj, args...)
					mangledName := fmt.Sprintf("%s_%s", typeName, methodName)

					// Lower receiver
					recvVal := ls.lowerExpr(fe.X)

					// Lower args
					var args []hir.Value
					args = append(args, recvVal) // receiver is first arg
					for _, a := range x.Args {
						args = append(args, ls.lowerExpr(a))
					}

					dst := ls.b.FreshTemp("call")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: mangledName, Args: args})
					// Consume args (methods move by default)
					for _, arg := range args {
						ls.consumeTemp(arg)
					}
					return dst
				}
			}
		}
	}

	// 3. M14: Struct Instantiation
	// Check if callee is a Type (Struct or Generic Instance)
	// We rely on the fact that check_inst resolves the call type to the struct type.
	if ls.info != nil {
		resT := ls.info.Types[x]
		var st *types.Struct

		if s, ok := resT.(*types.Struct); ok {
			st = s
		} else if g, ok := resT.(*types.Generic); ok {
			if s, ok := g.Base.(*types.Struct); ok {
				st = s
			}
		}

		if st != nil {
			// Check if the callee name matches the struct name (heuristic for constructor call)
			// Or just assume if the result type is a struct, it's a constructor call.
			// But it could be a function returning a struct.
			// We check if callee is an Ident that resolves to a Type symbol.
			isConstructor := false
			if id, ok := x.Callee.(*ast.Ident); ok {
				if sym := ls.info.Idents[id]; sym != nil && sym.Kind == check.SymType {
					isConstructor = true
				}
			}

			if isConstructor {
				// Emit Alloc
				// Calculate size
				size := 0
				for _, f := range st.Fields {
					size += getSize(f.Type)
				}
				// Align to 8 bytes for simplicity
				if size == 0 {
					size = 1
				} // Empty struct

				inst := ls.b.FreshTemp("inst")
				// Allocate as i8 array
				ls.b.Emit(&hir.Alloca{Type: "i8", Count: size, Dst: inst})

				// Initialize fields
				// We iterate ArgNodes to get names and values
				for _, arg := range x.ArgNodes {
					name := arg.Name.Name
					valExpr := arg.Expr
					val := ls.lowerExpr(valExpr)

					// Find field
					offset := 0
					var fieldType types.T
					for _, f := range st.Fields {
						if f.Name == name {
							fieldType = f.Type
							break
						}
						offset += getSize(f.Type)
					}

					// Emit GEP
					fieldPtr := ls.b.FreshTemp("field_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    inst,
						Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
						Dst:     fieldPtr,
					})

					// Check boxing (primitive -> Generic T (ptr))
					storageType := lowerType(fieldType)
					// We need to know the type of val.
					// We can look up valExpr type.
					valExprType := ls.info.Types[valExpr]
					valLowerType := lowerType(valExprType)

					if storageType == "ptr" && valLowerType != "ptr" && valLowerType != "void" {
						// Box: cast primitive to ptr
						boxed := ls.b.FreshTemp("boxed")
						ls.b.Emit(&hir.Cast{Dst: boxed, Src: val, Type: "ptr"})
						val = boxed
					}

					// Store
					ls.b.Emit(&hir.Store{
						Dst: fieldPtr,
						Val: val,
					})
				}
				return inst
			}
		}
	}

	// Legacy/Default behavior
	callee := ls.calleeName(x.Callee)
	var args []hir.Value
	for _, a := range x.Args {
		args = append(args, ls.lowerExpr(a))
	}

	// M15: Box arguments for generic functions
	// If the callee is a generic function (erased), we need to box primitive arguments to ptr
	if ls.info != nil {
		if id, ok := x.Callee.(*ast.Ident); ok {
			// Check if this is a generic function by looking at the original declaration
			isGeneric := false
			if set, ok := ls.info.Funcs[id.Name]; ok && len(set.Cands) > 0 {
				// Check the declaration (not the instantiated type)
				if set.Cands[0].Decl != nil && len(set.Cands[0].Decl.TypeParams) > 0 {
					isGeneric = true
				}
			}

			if isGeneric {
				// This is a call to a generic function
				// Box all primitive arguments to ptr (allocate + store + return pointer)
				for i, arg := range args {
					argType := "i32" // default
					if i < len(x.Args) {
						if t := ls.info.Types[x.Args[i]]; t != nil {
							argType = lowerType(t)
						}
					}

					if argType != "ptr" && argType != "void" {
						// Allocate storage
						boxPtr := ls.b.FreshTemp("arg_box_ptr")
						ls.b.Emit(&hir.Alloca{Type: argType, Count: 1, Dst: boxPtr})
						// Store value
						ls.b.Emit(&hir.Store{Dst: boxPtr, Val: arg})
						// Use pointer as argument
						args[i] = boxPtr
					}
				}
			}
		}
	}

	// Special-cases for arena helpers
	switch callee {
	case "arena.alloc":
		dst := ls.b.FreshTemp("alloc")
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "arena.alloc", Args: args})
		ls.tempsFromArenaAlloc[dst.Name] = true
		return dst
	case "arena.register_poll":
		ls.b.Emit(&hir.Call{Fn: "arena.register_poll", Args: args})
		return nil
	}

	dst := ls.b.FreshTemp("call")
	if callee == "" {
		callee = "<call>"
	}

	// Infer return type from type checker
	var retType string
	if ls.info != nil {
		if t := ls.info.Types[x]; t != nil {
			retType = lowerType(t)
			// Don't set void - let backend use defaults
			if retType == "void" {
				retType = ""
			}
		}
	}

	ls.b.Emit(&hir.Call{Dst: dst, Fn: callee, Args: args, Type: retType})

	// Consume args if not a known borrowing function
	// TODO: Use type checker info to determine if callee borrows
	if callee != "print" && callee != "asprintf" && callee != "bool_to_cstring" && !strings.HasPrefix(callee, "arena.") {
		for _, arg := range args {
			ls.consumeTemp(arg)
		}
	}

	return dst
}

// lowerType maps types to LLVM strings (Tier-0 subset).
func lowerType(t types.T) string {
	if t == nil {
		return "void"
	}

	// Handle struct types explicitly
	if _, ok := t.(*types.Struct); ok {
		return "ptr"
	}

	name := t.String()
	switch name {
	case "int", "i32", "u32":
		return "i32"
	case "i64", "u64", "isize", "usize":
		return "i64"
	case "bool":
		return "i1"
	case "float":
		return "double"
	case "str":
		return "ptr"
	case "none":
		return "void"
	}
	if strings.HasPrefix(name, "list[") {
		return "ptr" // list struct pointer
	}
	return "ptr" // default
}
