package lower

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
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
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, weakLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}, mutable: map[string]bool{}, types: map[string]types.T{}, tempDrops: map[string]bool{}, files: map[string]bool{}}}, // root
		terminated:          false,
		info:                info,
		src:                 nil,
		tempsFromArenaAlloc: map[string]bool{},
		nonOwnedTemps:       map[string]bool{},
		matchLocals:         map[string]hir.Value{},
	}
	ls.lowerBlock(blk)
	return b.Func()
}

// LowerBlockFromSource behaves like LowerBlock but can materialize string literals
// by scanning the original source using (line,col) from StrLit.Span.
// LowerFuncFromDecl lowers a function declaration to HIR, including its parameters.
// desiBuiltins are Desi compiler intrinsics with special lowering in lowerCall.
// These must NOT be mangled even when from-imported, because lowerCall checks
// calleeName == "print" etc. for builtin dispatch.
var desiBuiltins = map[string]bool{
	"print": true, "len": true, "str": true, "int": true, "float": true, "bool": true,
	"type_of": true, "assert": true, "open": true, "dbg": true,
	"sum": true, "min": true, "max": true, "any": true, "all": true,
	"sorted": true, "reversed": true, "zip": true, "enumerate": true,
	"map": true, "filter": true, "reduce": true, "foldl": true, "foldr": true,
	"range": true, "input": true, "hash": true, "id": true, "chr": true, "ord": true,
	"hex": true, "oct": true, "bin": true, "abs": true, "round": true, "pow": true, "todo": true,
	"set_recursion_limit": true, "is_reload": true, "reload_count": true,
	"state_file": true, "write_state": true, "read_state": true, "delete_state": true,
	"rc": true, "arc": true, "weak": true,
}

// mangleDesiName adds __desi$ prefix to a Desi function name.
// This is called on the DEFINITION side of module-imported functions
// to prevent their pub def wrappers from shadowing C/POSIX symbols.
// User-defined functions are NOT mangled (they need their original names
// for callbacks, lambdas, and hot-reload).
func mangleDesiName(name string) string {
	return "__desi$" + name
}

// overloadSymbol returns the symbol to emit for fd, given the name its
// definition would otherwise use.
//
// Overloads share a name, and a name is one symbol: emitting them all as `f`
// meant the backend kept the first definition and dropped the rest, while
// every call site — whichever overload the checker picked — called that one
// survivor. `f(1.0)` ran the int body.
//
// Each overload past the first therefore gets a suffix. The first keeps the
// plain name so that callbacks, lambdas and hot-reload, which look functions
// up by their source name, are unaffected for the overwhelmingly common case
// of a name declared once.
//
// Definitions and call sites both resolve through here, so they agree by
// construction. Ordering is the declaration order held in the candidate set,
// which is stable across a build.
func overloadSymbol(base string, fd *ast.FuncDecl, info *check.Info) string {
	if fd == nil || info == nil || isExternFunction(fd) {
		return base
	}
	set, ok := info.Funcs[fd.Name.Name]
	if !ok || set == nil || len(set.Cands) <= 1 {
		return base
	}
	decls := make([]*ast.FuncDecl, 0, len(set.Cands))
	for _, cand := range set.Cands {
		if cand != nil && cand.Decl != nil {
			decls = append(decls, cand.Decl)
		}
	}
	return overloadSymbolFor(base, fd, decls)
}

// overloadSymbolFor picks fd's symbol out of the set of same-named declarations.
//
// The index comes from source position, not from the order of the slice. A
// definition is lowered with the defining module's own type info, while a call
// resolves through the importer's view of that module's exports — two lists
// built independently. Ordering them by where they are written makes both
// sides agree without having to trust that those lists were assembled the
// same way, which is not something a symbol name should depend on.
func overloadSymbolFor(base string, fd *ast.FuncDecl, decls []*ast.FuncDecl) string {
	if fd == nil || len(decls) <= 1 {
		return base
	}
	sorted := append([]*ast.FuncDecl(nil), decls...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Span.Start.Line != sorted[j].Span.Start.Line {
			return sorted[i].Span.Start.Line < sorted[j].Span.Start.Line
		}
		return sorted[i].Span.Start.Col < sorted[j].Span.Start.Col
	})
	for i, d := range sorted {
		if d == fd || (d.Span.Start.Line == fd.Span.Start.Line && d.Span.Start.Col == fd.Span.Start.Col) {
			if i == 0 {
				return base
			}
			return fmt.Sprintf("%s$%d", base, i)
		}
	}
	return base
}

// moduleOverloadSymbol is the call-site counterpart for a function reached
// through an import: the importer has the module's declarations in
// ModuleExports rather than in its own Funcs table.
func moduleOverloadSymbol(base string, fd *ast.FuncDecl, info *check.Info) string {
	if fd == nil || info == nil || info.R == nil || isExternFunction(fd) {
		return base
	}
	for _, ex := range info.R.ModuleExports {
		if ex == nil {
			continue
		}
		if decls, ok := ex.FuncDecls[fd.Name.Name]; ok && len(decls) > 1 {
			for _, d := range decls {
				if d == fd || (d.Span.Start.Line == fd.Span.Start.Line && d.Span.Start.Col == fd.Span.Start.Col) {
					return overloadSymbolFor(base, fd, decls)
				}
			}
		}
	}
	return base
}

// isFromImportName reports whether name came in through `from mod import name`.
// Those definitions are mangled, so their call sites must be too.
func isFromImportName(info *check.Info, name string) bool {
	if info == nil || info.R == nil {
		return false
	}
	_, ok := info.R.FromItems[name]
	return ok
}

// isExternFunction checks if a function has @extern decorator
func isExternFunction(fd *ast.FuncDecl) bool {
	for _, dec := range fd.Decorators {
		if dec.Name.Name == "extern" {
			return true
		}
	}
	return false
}

// extractExternFromWrapper walks a pub def wrapper's body to find the @extern
// function it calls. Handles the common pattern:
//
//	pub def choice(items: list[int]) -> int:
//	    unsafe:
//	        return __random_choice_int(items)
//
// Returns the inner function name (e.g., "__random_choice_int"), or "" if not found.
//
// Inlining REPLACES the call site `wrapper(a, b, ...)` with `inner(a, b, ...)`,
// forwarding the wrapper's actual arguments verbatim. That is only sound when
// the wrapper is a *pure 1:1 pass-through*: a single meaningful statement whose
// inner call forwards exactly the wrapper's parameters, in declaration order.
// Otherwise the forwarded args would be mismatched — e.g. a multi-statement
// wrapper like `http.ws` (set_path(path); set_on_message(handler)) would drop
// the second call and pass `srv` where `__ws_set_path` expects `path`.
func extractExternFromWrapper(fd *ast.FuncDecl) string {
	if fd == nil || fd.Body == nil {
		return ""
	}

	// Collect meaningful (non-docstring) statements. A pure pass-through has
	// exactly one, containing the inner call.
	var meaningful []ast.Stmt
	for _, stmt := range fd.Body.Stmts {
		if _, isDoc := stmt.(*ast.DocStringStmt); isDoc {
			continue
		}
		meaningful = append(meaningful, stmt)
	}
	if len(meaningful) != 1 {
		return "" // multiple side effects — not a pass-through (e.g. http.ws)
	}

	name, args := extractCallWithArgsFromStmt(meaningful[0])
	if name == "" {
		return ""
	}

	// Arity must match exactly, and every argument must be the wrapper's
	// parameter at the same position (verbatim forward, no constants/reorder).
	if len(args) != len(fd.Params) {
		return ""
	}
	for i, arg := range args {
		id, ok := arg.(*ast.Ident)
		if !ok || id.Name != fd.Params[i].Name.Name {
			return ""
		}
	}
	return name
}

// extractCallWithArgsFromStmt returns the callee name and argument expressions
// of a single call statement (return/expr/unsafe-wrapped). Empty name if the
// statement is not a simple call to a named function.
func extractCallWithArgsFromStmt(stmt ast.Stmt) (string, []ast.Expr) {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		if s.Value != nil {
			if call, ok := s.Value.(*ast.CallExpr); ok {
				if id, ok := call.Callee.(*ast.Ident); ok {
					return id.Name, call.Args
				}
			}
		}
	case *ast.ExprStmt:
		if call, ok := s.Expr.(*ast.CallExpr); ok {
			if id, ok := call.Callee.(*ast.Ident); ok {
				return id.Name, call.Args
			}
		}
	case *ast.UnsafeBlock:
		if s.Body != nil {
			// An unsafe block wrapping a single call is still a pass-through.
			var meaningful []ast.Stmt
			for _, inner := range s.Body.Stmts {
				if _, isDoc := inner.(*ast.DocStringStmt); isDoc {
					continue
				}
				meaningful = append(meaningful, inner)
			}
			if len(meaningful) == 1 {
				return extractCallWithArgsFromStmt(meaningful[0])
			}
		}
	}
	return "", nil
}

// candidateFor returns the overload candidate that `fd` itself declared.
//
// info.Funcs is keyed by bare name, and a name does not map to a single
// declaration: overloads put several candidates under one key, and class
// methods are registered alongside module-level functions (check_type.go adds
// them so checkFunc can find their signatures). Taking Cands[0] therefore
// lowers a declaration with some *other* declaration's signature.
//
// That produced invalid IR rather than a diagnostic. A module-level
// `tag(x: int) -> int` sharing its name with a method `Box.tag(self) -> Box`
// was lowered with the method's types and emitted `add ptr %x, 1`.
//
// Candidates without a Decl (builtins, cross-module exports) cannot be matched
// by identity, so an unmatched lookup keeps the previous first-candidate
// behaviour.
func candidateFor(info *check.Info, fd *ast.FuncDecl) *check.FuncCand {
	if info == nil || fd == nil {
		return nil
	}
	set, ok := info.Funcs[fd.Name.Name]
	if !ok || len(set.Cands) == 0 {
		return nil
	}
	for _, cand := range set.Cands {
		if cand != nil && cand.Decl == fd {
			return cand
		}
	}
	return set.Cands[0]
}

// LowerFuncFromDecl lowers a function declaration to HIR.
// Used for user-defined functions — does NOT mangle names.
func LowerFuncFromDecl(fd *ast.FuncDecl, info *check.Info, src []byte, globals map[string]bool) *hir.Func {
	return lowerFuncFromDeclWithContext(fd, info, src, "", nil, globals, false)
}

// LowerFuncFromDeclEx lowers a function declaration with an option to force-mangle.
// Used for imported module functions — always mangles non-extern definitions to
// prevent pub def wrappers from shadowing C/POSIX symbols.
func LowerFuncFromDeclEx(fd *ast.FuncDecl, info *check.Info, src []byte, globals map[string]bool, forceMangleModule bool) *hir.Func {
	return lowerFuncFromDeclWithContext(fd, info, src, "", nil, globals, forceMangleModule)
}

// LowerFuncForDunderNew lowers a __new__ method with context to prevent recursive constructor calls
func LowerFuncForDunderNew(fd *ast.FuncDecl, info *check.Info, src []byte, className string, selfPtr hir.Value, globals map[string]bool) *hir.Func {
	return lowerFuncFromDeclWithContext(fd, info, src, className, selfPtr, globals, false)
}

func lowerFuncFromDeclWithContext(fd *ast.FuncDecl, info *check.Info, src []byte, dunderNewClass string, selfPtr hir.Value, globals map[string]bool, forceMangleModule bool) *hir.Func {
	// Determine function name for LLVM IR
	funcName := fd.Name.Name
	if !isExternFunction(fd) && forceMangleModule {
		// Module-imported function: always mangle to avoid C symbol collisions
		funcName = mangleDesiName(funcName)
	}
	funcName = overloadSymbol(funcName, fd, info)
	b := hir.NewFunc(funcName)
	// Build param names map BEFORE lowering so isParam works during body lowering
	paramNames := make(map[string]bool, len(fd.Params))
	for _, p := range fd.Params {
		if p.Name.Name != "" {
			paramNames[p.Name.Name] = true
		}
	}
	ls := &lowerState{
		b:                   b,
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, weakLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}, mutable: map[string]bool{}, types: map[string]types.T{}, tempDrops: map[string]bool{}, files: map[string]bool{}}}, // root
		terminated:          false,
		info:                info,
		src:                 src,
		globals:             globals,
		paramNames:          paramNames,
		tempsFromArenaAlloc: map[string]bool{},
		nonOwnedTemps:       map[string]bool{},
		matchLocals:         map[string]hir.Value{},
		strAccums:           strAccumCandidates(fd),
		inDunderNew:         dunderNewClass != "",
		dunderNewClass:      dunderNewClass,
		dunderNewSelf:       selfPtr,
	}

	// Task 4: Initialize local arena if we have non-escaping local variables
	if hasNonEscapingLocals(fd.Body, info) {
		arenaPtr := ls.b.FreshTemp("local_arena")
		ls.b.Emit(&hir.Call{
			Dst:  arenaPtr,
			Fn:   "__arena_new",
			Args: []hir.Value{hir.ConstInt{Text: "0", Type: "i64"}},
			Type: "ptr",
		})
		ls.localArena = arenaPtr
	}

	// Populate parameter types into the root scope
	if info != nil {
		if cand := candidateFor(info, fd); cand != nil {
			funcType := cand.Type
			if funcType != nil {
				for i, p := range fd.Params {
					if i < len(funcType.Params) && funcType.Params[i] != nil {
						ls.scopes[0].types[p.Name.Name] = funcType.Params[i]
					}
				}
			}
		}
	}

	// Set return type BEFORE lowering body (needed for return type coercion)
	f := b.Func()
	if info != nil {
		if cand := candidateFor(info, fd); cand != nil {
			funcType := cand.Type
			if funcType != nil && funcType.Ret != nil {
				retType := lowerType(funcType.Ret)
				if retType != "void" {
					f.RetType = retType
				}
			}
		}
	}
	if f.RetType == "" && fd.RetType != nil {
		switch fd.RetType.Name {
		case "str":
			f.RetType = "ptr"
		case "int":
			f.RetType = "i32"
		case "bool":
			f.RetType = "i1"
		case "float":
			f.RetType = "double"
		case "u64", "i64":
			f.RetType = "i64"
		case "none":
			f.RetType = "void"
		default:
			f.RetType = "ptr"
		}
	}

	ls.lowerBlock(fd.Body)
	f.Origin = fd // Track AST origin for move analysis lookup

	// Populate parameters from AST
	// For variadic functions, the last parameter has already been wrapped in list[T] by the type checker
	// We need to get the types from the type checker's info
	if info != nil {
		// Get the signature this declaration itself contributed.
		if cand := candidateFor(info, fd); cand != nil {
			funcType := cand.Type
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

	// Fallback: use AST type annotations when type info is not available
	for _, p := range fd.Params {
		paramType := "ptr" // default
		if p.Type != nil {
			paramType = llvmTypeFromAST(p.Type.Name)
		}
		f.Params = append(f.Params, hir.Param{Name: p.Name.Name, Type: paramType})
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

	// Check for @inline decorator
	for _, dec := range fd.Decorators {
		if dec.Name.Name == "inline" {
			f.Inline = true
			break
		}
	}

	return f
}

// llvmTypeFromAST converts an AST type name to an LLVM type string.
func llvmTypeFromAST(name string) string {
	switch name {
	case "int":
		return "i32"
	case "bool":
		return "i1"
	case "float", "f64":
		return "double"
	case "f32":
		return "float"
	case "str":
		return "ptr"
	case "usize", "isize", "i64", "u64":
		return "i64"
	case "i8", "u8":
		return "i8"
	case "i16", "u16":
		return "i16"
	case "i32", "u32":
		return "i32"
	default:
		return "ptr" // custom types are pointers
	}
}

// LowerBlockFromSource lowers just a block with a given name (for compatibility with existing code).
func LowerBlockFromSource(name string, blk *ast.Block, info *check.Info, src []byte) *hir.Func {
	b := hir.NewFunc(name)
	ls := &lowerState{
		b:                   b,
		scopes:              []*scope{{locals: []string{}, rcLike: map[string]bool{}, weakLike: map[string]bool{}, moved: map[string]bool{}, arenas: map[string]bool{}, arenaOwned: map[string]bool{}, mutable: map[string]bool{}, types: map[string]types.T{}, tempDrops: map[string]bool{}, files: map[string]bool{}}}, // root
		terminated:          false,
		info:                info,
		src:                 src,
		tempsFromArenaAlloc: map[string]bool{},
		nonOwnedTemps:       map[string]bool{},
		matchLocals:         map[string]hir.Value{},
	}
	ls.lowerBlock(blk)
	return b.Func()
}

// LowerDefaultToStr generates a default to_str implementation.
// If reprFuncName is non-empty, delegate to that function (e.g., __repr__).
// Otherwise, return the type name as a string.
func LowerDefaultToStr(name, typeName string, reprFuncName string) *hir.Func {
	b := hir.NewFunc(name)
	f := b.Func()
	f.Params = []hir.Param{{Name: "self", Type: "ptr"}}
	f.RetType = "ptr"

	if reprFuncName != "" {
		// Delegate to __repr__
		res := b.FreshTemp("repr_result")
		b.Emit(&hir.Call{Dst: res, Fn: reprFuncName, Args: []hir.Value{hir.Var{Name: "self"}}, Type: "ptr"})
		b.Emit(&hir.Ret{Val: res})
	} else {
		// Fall back to __desi_default_repr(self, type_name) → "<TypeName at 0xADDR>"
		res := b.FreshTemp("repr_result")
		b.Emit(&hir.Call{Dst: res, Fn: "__desi_default_repr", Args: []hir.Value{hir.Var{Name: "self"}, hir.ConstStr{Text: typeName}}, Type: "ptr"})
		b.Emit(&hir.Ret{Val: res})
	}
	return f
}

type lowerState struct {
	b           *hir.Builder
	scopes      []*scope // stack
	terminated  bool     // set once a return is emitted
	info        *check.Info
	src         []byte                 // optional: original source for literal materialization
	globals     map[string]bool        // names of global variables
	globalTypes map[string]types.T     // types of global variables (for field access)
	enums       map[string]*types.Enum // user-defined enum declarations
	paramNames  map[string]bool        // parameter names (populated before body lowering)

	tempsFromArenaAlloc map[string]bool      // temp.Name -> true if produced by ArenaAlloc
	nonOwnedTemps       map[string]bool      // temp.Name -> true if the value is a payload alias or stack slot (never drop)
	matchLocals         map[string]hir.Value // pattern binding variables (name -> HIR value)
	tempSuppress        int                  // >0: don't register temp drops (expression-level control flow; see temp_tracking.go)
	strAccums           map[string]bool      // mutable string locals with owned-accumulator lowering (see str_accum.go)

	// __new__ method context: when inside a user-defined __new__,
	// ClassName(field=val) should initialize self, not allocate new instance
	inDunderNew    bool      // true when lowering inside a __new__ method body
	dunderNewClass string    // class name for the current __new__
	dunderNewSelf  hir.Value // the self pointer to initialize

	// TaskGroup wrapper functions: wrapperName -> emitted (to avoid duplicates)
	emittedWrappers map[string]bool

	// try/except block context: when inside a try block, the ? operator
	// redirects Err to the except handler instead of doing early return
	inTryBlock  bool       // true when lowering inside a try body
	exceptBlock *hir.Block // the except handler block to jump to on Err
	tryErrSlot  hir.Value  // alloca'd slot to store the error value for except

	// Task 4: Escape Analysis & Automatic Function-Local Arenas
	localArena        hir.Value // Function-scoped arena pointer if initialized
	rewindStack       []bool    // per enclosing loop: did it take an arena mark?
	currentAllocArena hir.Value // The arena to direct allocations to during RHS lowering

	// loops is a stack of the enclosing loops, innermost last, so that 'break'
	// and 'continue' can name a concrete target block. Every loop form — while
	// and each of the for desugarings — pushes one entry.
	loops []loopTargets
}

// loopTargets are the two blocks a loop-jump statement can branch to, plus the
// scope depth at the point the loop was entered.
type loopTargets struct {
	cont *hir.Block // run the latch, then re-test the condition: 'continue'
	exit *hir.Block // leave the loop: 'break'
	// depth is len(scopes) just after the loop pushed its own scope. A break or
	// continue drops every scope opened at or beyond it, because the drops the
	// body emits at its tail are never reached from the jump.
	depth int
}

// pushLoop records the jump targets for a loop body about to be lowered. Call
// it after the loop has pushed its own scope, and pair it with popLoop.
func (ls *lowerState) pushLoop(cont, exit *hir.Block) {
	ls.loops = append(ls.loops, loopTargets{cont: cont, exit: exit, depth: len(ls.scopes)})
}

func (ls *lowerState) popLoop() {
	if len(ls.loops) > 0 {
		ls.loops = ls.loops[:len(ls.loops)-1]
	}
}

// innermostLoop returns the loop a break or continue belongs to.
func (ls *lowerState) innermostLoop() (loopTargets, bool) {
	if len(ls.loops) == 0 {
		return loopTargets{}, false
	}
	return ls.loops[len(ls.loops)-1], true
}

// emitLoopExitDrops drops the scopes a break or continue is jumping out of,
// innermost first. The scope stack itself is left alone — lowering of the rest
// of the body continues after this statement, and the enclosing loop form pops
// its own scope as usual.
func (ls *lowerState) emitLoopExitDrops() {
	lt, ok := ls.innermostLoop()
	if !ok {
		return
	}
	for i := len(ls.scopes) - 1; i >= lt.depth-1 && i >= 0; i-- {
		ls.emitScopeDrops(ls.scopes[i])
	}
}

// callsGenericFunc reports whether a call resolves to a generic function, whose
// result therefore comes back boxed behind a pointer.
func (ls *lowerState) callsGenericFunc(call *ast.CallExpr) bool {
	if ls.info == nil {
		return false
	}
	if chosen := ls.info.ChosenOverloads[call]; chosen != nil {
		if (chosen.Decl != nil && len(chosen.Decl.TypeParams) > 0) ||
			(chosen.ModuleDecl != nil && len(chosen.ModuleDecl.TypeParams) > 0) {
			return true
		}
	}
	// The checker does not record a winner for every call, so fall back to the
	// declaration the name resolves to.
	id, ok := call.Callee.(*ast.Ident)
	if !ok {
		return false
	}
	set, ok := ls.info.Funcs[id.Name]
	if !ok || len(set.Cands) == 0 {
		return false
	}
	cand := set.Cands[0]
	return (cand.Decl != nil && len(cand.Decl.TypeParams) > 0) ||
		(cand.ModuleDecl != nil && len(cand.ModuleDecl.TypeParams) > 0)
}

// unboxGenericResult loads the value back out of the box a generic function
// returns, when the type the call was instantiated at is a primitive.
//
// A generic function is emitted once for every T, so it hands its result back
// behind a pointer. Unboxing used to be done by the `let` lowering alone, which
// meant it only happened when the result was bound to a variable: `let x =
// ident(42)` was fine, and `str(ident(42))` handed the backend a raw pointer.
// Nothing downstream could tell what was inside it, so `str` fell through to a
// synthesized `Unknown_to_str` and the program failed at *link* time — long
// after `desic check` had said ok.
//
// Doing it here, where the call is lowered, covers every position a call can
// appear in: an argument, a return, an operand, a collection element.
func (ls *lowerState) unboxGenericResult(call *ast.CallExpr, v hir.Value) hir.Value {
	if v == nil || ls.info == nil || !ls.callsGenericFunc(call) {
		return v
	}
	t := ls.info.Types[call]
	if t == nil {
		return v
	}
	lowered := lowerType(t)
	if lowered == "ptr" || lowered == "void" || lowered == "" {
		// A pointer-shaped T needs no unboxing: the box holds the value itself.
		return v
	}
	unboxed := ls.b.FreshTemp("unboxed")
	ls.b.Emit(&hir.Load{Type: lowered, Src: v, Dst: unboxed})
	return unboxed
}

// printLock and printUnlock bracket the output calls one print statement emits,
// so a line cannot be torn by another task printing at the same time. A print
// lowers to several calls — one per value, plus separators and the terminator —
// and each is atomic on its own while the sequence is not.
//
// Take the lock only once every argument has been evaluated into a temp. An
// argument is arbitrary user code and may block (`print(ch.recv())`); holding
// this lock across it would stall every other task's print behind it, turning a
// cosmetic problem into a deadlock. The runtime lock is recursive anyway,
// because a to_str dunder reached during emission can print.
func (ls *lowerState) printLock() {
	ls.b.Emit(&hir.Call{Dst: ls.b.FreshTemp("printlk"), Fn: "__desi_print_lock", Args: []hir.Value{}})
}

func (ls *lowerState) printUnlock() {
	ls.b.Emit(&hir.Call{Dst: ls.b.FreshTemp("printulk"), Fn: "__desi_print_unlock", Args: []hir.Value{}})
}

// closeLoopBody ends a lowered loop body: on the path that falls off the end it
// drops what the body scope owns and branches to the latch, then pops the loop
// and its scope and positions the builder in the latch.
//
// The drops belong here, on the edge, and not in the latch — even though the
// latch is the one block every iteration passes through. A break or continue
// jumps out from the middle of the body, so the latch is not dominated by the
// body's tail: freeing a temp there that only the fall-through path defines is
// invalid IR ("instruction does not dominate all uses"). Those jumps drop what
// is live at the jump themselves, via emitLoopExitDrops, which is also why
// repeating the drops in the latch would free twice on that path.
//
// What the latch does own is the loop's own bookkeeping — for a desugared 'for',
// the index increment — which is why continue targets it rather than the
// condition block.
func (ls *lowerState) closeLoopBody(latch *hir.Block) {
	if !ls.terminated {
		ls.emitScopeDrops(ls.cur())
		ls.b.Emit(&hir.Jump{Target: latch})
	}
	ls.popLoop()
	ls.pop()
	// A break or continue leaves ls.terminated set; the latch is a fresh block
	// still reachable from them, so clear it before filling.
	ls.terminated = false
	ls.b.SetBlock(latch)
	// Hand this iteration's arena bytes back, before the latch's own
	// bookkeeping. The latch is the right place for exactly the reason the drops
	// are not: every iteration passes through it, by falling off the end of the
	// body or by 'continue', and neither carries anything out. 'break' goes to
	// the exit block instead, which releases the mark outright.
	ls.emitArenaRewind()
}

// beginRewindableLoop marks the arena before a loop whose body allocations the
// checker proved are all dead by the end of each iteration. It reports whether
// it did, so the caller knows to release afterwards.
func (ls *lowerState) beginRewindableLoop(s ast.Stmt) bool {
	if ls.localArena == nil || ls.info == nil || !ls.info.RewindableLoops[s] {
		ls.rewindStack = append(ls.rewindStack, false)
		return false
	}
	ls.b.Emit(&hir.Call{
		Fn:   "__arena_mark",
		Args: []hir.Value{ls.localArena},
		Type: "void",
	})
	ls.rewindStack = append(ls.rewindStack, true)
	return true
}

// endRewindableLoop retires the mark. It runs on the loop's exit block, so it
// covers leaving by 'break' as well as running out of iterations. Leaving by
// 'return' skips it, which is harmless: that path destroys the arena entirely.
func (ls *lowerState) endRewindableLoop() {
	n := len(ls.rewindStack)
	if n == 0 {
		return
	}
	active := ls.rewindStack[n-1]
	ls.rewindStack = ls.rewindStack[:n-1]
	if !active || ls.localArena == nil {
		return
	}
	ls.b.Emit(&hir.Call{
		Fn:   "__arena_release",
		Args: []hir.Value{ls.localArena},
		Type: "void",
	})
}

// emitArenaRewind returns the arena to the innermost mark, if the loop being
// closed took one.
func (ls *lowerState) emitArenaRewind() {
	n := len(ls.rewindStack)
	if n == 0 || !ls.rewindStack[n-1] || ls.localArena == nil {
		return
	}
	ls.b.Emit(&hir.Call{
		Fn:   "__arena_rewind",
		Args: []hir.Value{ls.localArena},
		Type: "void",
	})
}

type scope struct {
	locals      []string        // in declaration order
	rcLike      map[string]bool // locals that are rc/arc
	weakLike    map[string]bool // locals that are weak
	moved       map[string]bool // locals moved-from; skip drop
	borrowed    map[string]bool // locals that are borrowed/aliased; skip drop
	defers      []hir.Value
	arenas      map[string]bool    // names that are arena handles in this scope
	arenaOwned  map[string]bool    // locals whose storage originates from arena.alloc
	mutable     map[string]bool    // variables declared with mut
	types       map[string]types.T // variable types for Drop
	tempDrops   map[string]bool    // temporaries that need to be dropped
	closers     map[string]string  // RAII: variables that need __close__ called (mangled name)
	files       map[string]bool    // RAII: file handles that need file_close at scope end
	mutexGuards map[string]bool    // RAII: mutex guards that need mutex_unlock at scope end
	readGuards  map[string]bool    // RAII: read guards that need read_guard_unlock at scope end
	writeGuards map[string]bool    // RAII: write guards that need write_guard_unlock at scope end
	senders     map[string]bool    // RAII: channel senders that need sender_drop at scope end
	receivers   map[string]bool    // RAII: channel receivers that need receiver_drop at scope end
	taskGroups  map[string]bool    // RAII: task groups that need taskgroup_destroy at scope end
}

func (ls *lowerState) push() {
	ls.scopes = append(ls.scopes, &scope{
		locals:      []string{},
		rcLike:      map[string]bool{},
		weakLike:    map[string]bool{},
		moved:       map[string]bool{},
		borrowed:    map[string]bool{},
		tempDrops:   map[string]bool{},
		defers:      []hir.Value{},
		arenas:      map[string]bool{},
		arenaOwned:  map[string]bool{},
		mutable:     map[string]bool{},
		types:       map[string]types.T{},
		closers:     map[string]string{},
		files:       map[string]bool{},
		mutexGuards: map[string]bool{},
		readGuards:  map[string]bool{},
		writeGuards: map[string]bool{},
		senders:     map[string]bool{},
		receivers:   map[string]bool{},
		taskGroups:  map[string]bool{},
	})
}
func (ls *lowerState) pop() *scope {
	last := ls.scopes[len(ls.scopes)-1]
	ls.scopes = ls.scopes[:len(ls.scopes)-1]
	return last
}
func (ls *lowerState) cur() *scope { return ls.scopes[len(ls.scopes)-1] }

// emitTaskGroupWrapper registers a wrapper function to be emitted.
// Uses global registry so LLVM backend can emit the wrappers.
func (ls *lowerState) emitTaskGroupWrapper(wrapperName, targetFnName string, numCaptures int, capTypes []string) {
	// Initialize map if needed (for local dedup)
	if ls.emittedWrappers == nil {
		ls.emittedWrappers = make(map[string]bool)
	}

	// Skip if already tracked locally
	if ls.emittedWrappers[wrapperName] {
		return
	}
	ls.emittedWrappers[wrapperName] = true

	// Register in global registry for LLVM backend
	RegisterTGWrapper(wrapperName, targetFnName, numCaptures, capTypes)
}

// ---- lowering ----
// NOTE: lowerBlock and lowerStmt have been moved to lower_stmt.go

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
	for i < len(src) && c < col {
		if src[i] == '\n' {
			return -1
		}
		_, sz := utf8.DecodeRune(src[i:])
		c++
		i += sz
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
		// Check for globals
		if ls.globals != nil && ls.globals[e.Name] && !ls.hasLocal(e.Name) {
			return "@" + e.Name
		}
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

func (ls *lowerState) calleeName(e ast.Expr, callExpr ...*ast.CallExpr) string {
	switch x := e.(type) {
	case *ast.Ident:
		// First check if this is a lambda variable (e.g., "double" -> "__lam$0")
		if ls.info != nil && ls.info.LambdaAliases != nil {
			if hiddenFunc, ok := ls.info.LambdaAliases[x.Name]; ok {
				return hiddenFunc
			}
		}
		// Check if this is an aliased import (e.g., "sum" from "from math import add as sum")
		if ls.info != nil && ls.info.ImportAliases != nil {
			if actualName, ok := ls.info.ImportAliases[x.Name]; ok {
				// Now check if the actual function is @extern - if so, use the @extern function name
				if set, ok := ls.info.Funcs[actualName]; ok && len(set.Cands) > 0 {
					cand := set.Cands[0]
					if cand.Extern && cand.Decl != nil {
						// The @extern function's declared name IS the C function name
						return cand.Decl.Name.Name
					}
				}
				return actualName
			}
		}
		// Check if this function is @extern - if so, use the declared name directly
		if ls.info != nil {
			// First, check ChosenOverloads — this handles from-import calls
			// where the function came from a module (e.g., from math import sin)
			if len(callExpr) > 0 && callExpr[0] != nil {
				if chosen, ok := ls.info.ChosenOverloads[callExpr[0]]; ok {
					decl := chosen.Decl
					if decl == nil {
						decl = chosen.ModuleDecl
					}
					if decl != nil {
						if isExternFunction(decl) {
							return decl.Name.Name
						}
						// For pub def wrappers, find the @extern function they call
						if innerName := extractExternFromWrapper(decl); innerName != "" {
							return innerName
						}
						// Locally declared functions keep their source name —
						// only module imports are mangled — but still have to
						// name the overload the checker picked. Every call used
						// to land on the bare name, i.e. whichever overload the
						// backend happened to emit first.
						if chosen.ModuleDecl == nil && !isFromImportName(ls.info, x.Name) {
							return overloadSymbol(decl.Name.Name, decl, ls.info)
						}
						return moduleOverloadSymbol(mangleDesiName(decl.Name.Name), decl, ls.info)
					}
				}
			}

			if set, ok := ls.info.Funcs[x.Name]; ok && len(set.Cands) > 0 {
				cand := set.Cands[0]
				if cand.Extern && cand.Decl != nil {
					// The @extern function's declared name IS the C function name
					return cand.Decl.Name.Name
				}
				// Check if this is an imported module function (has ModuleDecl)
				decl := cand.Decl
				if decl == nil {
					decl = cand.ModuleDecl
				}
				if decl != nil && cand.ModuleDecl != nil {
					// This function came from a module — resolve to @extern or mangle
					if innerName := extractExternFromWrapper(decl); innerName != "" {
						return innerName
					}
					return mangleDesiName(decl.Name.Name)
				}
			}
		}
		// If this is a from-import function, its definition is mangled — match it.
		// Only mangle lowercase names (functions). Uppercase names are classes
		// (Mutex, Channel, etc.) which have separate lowering paths.
		// Also exclude Desi builtins (print, len, etc.) which have special
		// lowering in lowerCall and must keep their original names.
		if ls.info != nil && ls.info.R != nil {
			if _, isFromImport := ls.info.R.FromItems[x.Name]; isFromImport {
				if len(x.Name) > 0 && x.Name[0] >= 'a' && x.Name[0] <= 'z' && !desiBuiltins[x.Name] {
					return mangleDesiName(x.Name)
				}
			}
		}
		// Local function call — use original name (user functions are NOT mangled)
		return x.Name
	case *ast.FieldExpr:
		// Check if this is a module-qualified call (e.g., "math.add")
		if ls.info != nil && ls.info.R != nil {
			// Check if the receiver (X) is a module name
			if id, ok := x.X.(*ast.Ident); ok {
				if _, isModule := ls.info.R.Imports[id.Name]; isModule {
					// It's a qualified call like time.strftime
					funcName := x.Name.Name

					// Check if the type checker chose a specific overload for this call
					if len(callExpr) > 0 && callExpr[0] != nil {
						if chosen, ok := ls.info.ChosenOverloads[callExpr[0]]; ok {
							// Try Decl first, then ModuleDecl
							decl := chosen.Decl
							if decl == nil {
								decl = chosen.ModuleDecl
							}
							if decl != nil {
								if isExternFunction(decl) {
									return decl.Name.Name
								}
								// For pub def wrappers, find the @extern function they call
								if innerName := extractExternFromWrapper(decl); innerName != "" {
									return innerName
								}
								// Overloads of a module function are emitted under
								// suffixed symbols, so the call has to name the one
								// the checker picked. Without this, http.serve(srv,
								// handler) called the one-argument serve(srv): the
								// server ran and silently ignored the handler.
								return moduleOverloadSymbol(mangleDesiName(decl.Name.Name), decl, ls.info)
							}
						}
					}

					// Fallback: check first candidate
					if set, ok := ls.info.Funcs[funcName]; ok && len(set.Cands) > 0 {
						cand := set.Cands[0]
						if cand.Extern {
							return funcName // @extern: use unmangled name
						}
					}
					// Non-extern: mangle to match definition
					return mangleDesiName(funcName)
				}
			}
		}
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
		if sc.borrowed[name] {
			continue // borrowed/aliased values are owned elsewhere; don't free
		}
		if sc.rcLike[name] {
			ls.b.Emit(&hir.DecRef{Val: hir.Var{Name: name}})
		} else if sc.weakLike[name] {
			ls.b.Emit(&hir.Call{Fn: "__weak_dec", Args: []hir.Value{hir.Var{Name: name}}, Type: "void"})
		} else {
			ls.b.Emit(&hir.Drop{Val: hir.Var{Name: name}, Type: sc.types[name]})
		}
	}
	// defers
	for i := len(sc.defers) - 1; i >= 0; i-- {
		v := sc.defers[i]
		// RAII closer?
		if varName, ok := v.(hir.Var); ok {
			if mangledName, exists := sc.closers[varName.Name]; exists {
				// Emit call to ClassName___close__(self)
				ls.b.Emit(&hir.Call{
					Fn:   mangledName,
					Args: []hir.Value{v},
					Type: "void", // __close__ returns none -> void
				})
				continue
			}
		}

		// Arena handle?
		if varName, ok := v.(hir.Var); ok && sc.arenas[varName.Name] {
			ls.b.Emit(&hir.DestroyArena{Arena: v})
			continue
		}
		// File handle? Call file_close
		if varName, ok := v.(hir.Var); ok && sc.files[varName.Name] {
			ls.b.Emit(&hir.Call{
				Fn:   "file_close",
				Args: []hir.Value{v},
				Type: "void",
			})
			continue
		}
		// MutexGuard? Call mutex_unlock to release the lock
		if varName, ok := v.(hir.Var); ok && sc.mutexGuards[varName.Name] {
			ls.b.Emit(&hir.Call{
				Fn:   "mutex_unlock",
				Args: []hir.Value{v},
				Type: "void",
			})
			continue
		}
		// ReadGuard? Call read_guard_unlock to release the read lock
		if varName, ok := v.(hir.Var); ok && sc.readGuards[varName.Name] {
			ls.b.Emit(&hir.Call{
				Fn:   "read_guard_unlock",
				Args: []hir.Value{v},
				Type: "void",
			})
			continue
		}
		// WriteGuard? Call write_guard_unlock to release the write lock
		if varName, ok := v.(hir.Var); ok && sc.writeGuards[varName.Name] {
			ls.b.Emit(&hir.Call{
				Fn:   "write_guard_unlock",
				Args: []hir.Value{v},
				Type: "void",
			})
			continue
		}
		// Sender? Call sender_drop to cleanup channel sender
		if varName, ok := v.(hir.Var); ok && sc.senders[varName.Name] {
			ls.b.Emit(&hir.Call{
				Fn:   "sender_drop",
				Args: []hir.Value{v},
				Type: "void",
			})
			continue
		}
		// Receiver? Call receiver_drop to cleanup channel receiver
		if varName, ok := v.(hir.Var); ok && sc.receivers[varName.Name] {
			ls.b.Emit(&hir.Call{
				Fn:   "receiver_drop",
				Args: []hir.Value{v},
				Type: "void",
			})
			continue
		}
		// TaskGroup? taskgroup_destroy waits for pending tasks, then frees.
		// (Previously fell through to the arena fallback — __arena_destroy
		// walked the TaskGroup as if it were a DesiArena and corrupted the heap.)
		if varName, ok := v.(hir.Var); ok && sc.taskGroups[varName.Name] {
			ls.b.Emit(&hir.Call{
				Fn:   "taskgroup_destroy",
				Args: []hir.Value{v},
				Type: "void",
			})
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
	ls.emitTempDrops(sc)
}

func (ls *lowerState) emitAllDefersAndDrops() {
	// Emit all pending defers and locals from inner-most to outer-most.
	for i := len(ls.scopes) - 1; i >= 0; i-- {
		ls.emitScopeDrops(ls.scopes[i])
	}
	// Task 4: destroy local arena at function exit
	if ls.localArena != nil {
		ls.b.Emit(&hir.DestroyArena{Arena: ls.localArena})
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

func (ls *lowerState) lookupLocalType(name string) types.T {
	for i := len(ls.scopes) - 1; i >= 0; i-- {
		if t, ok := ls.scopes[i].types[name]; ok {
			return t
		}
	}
	return nil
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

// markNonOwnedResult records that a lowered expression's value is a payload
// alias or stack-allocated slot — a `let` binding it must never be dropped
// (freeing stack memory or another owner's storage crashes).
func (ls *lowerState) markNonOwnedResult(v hir.Value) {
	if t, ok := v.(hir.Temp); ok {
		ls.nonOwnedTemps[t.Name] = true
	}
}

// isNonOwnedResult reports whether the value was marked by markNonOwnedResult.
func (ls *lowerState) isNonOwnedResult(v hir.Value) bool {
	if t, ok := v.(hir.Temp); ok {
		return ls.nonOwnedTemps[t.Name]
	}
	return false
}

func isRcLike(t types.T) bool {
	switch t.(type) {
	case *types.Rc, *types.Arc:
		return true
	default:
		return false
	}
}

func isWeakLike(t types.T) bool {
	switch t.(type) {
	case *types.Weak:
		return true
	default:
		return false
	}
}

// emitAlloc emits an arena-backed allocation if currentAllocArena is active, otherwise standard malloc.
func (ls *lowerState) emitAlloc(dst hir.Temp, size hir.Value) {
	if ls.currentAllocArena != nil {
		ls.b.Emit(&hir.ArenaAlloc{
			Dst:   dst,
			Arena: ls.currentAllocArena,
			Args:  []hir.Value{size},
		})
		ls.tempsFromArenaAlloc[dst.Name] = true
	} else {
		ls.b.Emit(&hir.Call{
			Dst:  dst,
			Fn:   "malloc",
			Args: []hir.Value{size},
			Type: "ptr",
		})
	}
}

// hasNonEscapingLocals recursively checks if a block contains any variables marked as non-escaping.
func hasNonEscapingLocals(body *ast.Block, info *check.Info) bool {
	if info == nil || len(info.NonEscaping) == 0 || body == nil {
		return false
	}
	found := false
	var inspect func(ast.Node)
	inspect = func(node ast.Node) {
		if node == nil || found {
			return
		}
		val := reflect.ValueOf(node)
		if val.Kind() == reflect.Ptr && val.IsNil() {
			return
		}
		switch x := node.(type) {
		case *ast.Block:
			for _, s := range x.Stmts {
				inspect(s)
			}
		case *ast.LetStmt:
			if x.Name.Name != "" && info.NonEscaping[&x.Name] {
				found = true
				return
			}
			for i := range x.Pattern {
				if info.NonEscaping[&x.Pattern[i]] {
					found = true
					return
				}
			}
			inspect(x.Value)
		case *ast.AssignStmt:
			for _, rhs := range x.RHS {
				inspect(rhs)
			}
		case *ast.ExprStmt:
			inspect(x.Expr)
		case *ast.ReturnStmt:
			inspect(x.Value)
		case *ast.IfStmt:
			inspect(x.Cond)
			inspect(x.Then)
			for _, elif := range x.Elifs {
				inspect(elif.Cond)
				inspect(elif.Body)
			}
			inspect(x.Else)
		case *ast.WhileStmt:
			inspect(x.Cond)
			inspect(x.Body)
		case *ast.ForStmt:
			inspect(x.Iter)
			inspect(x.Body)
		case *ast.CallExpr:
			inspect(x.Callee)
			for _, arg := range x.Args {
				inspect(arg)
			}
		}
	}
	inspect(body)
	return found
}

// emitListNew emits a list allocation call. If currentAllocArena is active, it calls list_new_in.
func (ls *lowerState) emitListNew(dst hir.Temp, typeTag, toStrFunc hir.Value) {
	if ls.currentAllocArena != nil {
		ls.b.Emit(&hir.Call{
			Dst:  dst,
			Fn:   "list_new_in",
			Args: []hir.Value{ls.currentAllocArena, typeTag, toStrFunc},
			Type: "ptr",
		})
	} else {
		ls.b.Emit(&hir.Call{
			Dst:  dst,
			Fn:   "list_new",
			Args: []hir.Value{typeTag, toStrFunc},
			Type: "ptr",
		})
	}
}

// emitDictNew emits a dict allocation call. If currentAllocArena is active, it calls dict_new_in.
func (ls *lowerState) emitDictNew(dst hir.Temp, args []hir.Value) {
	if ls.currentAllocArena != nil {
		allArgs := append([]hir.Value{ls.currentAllocArena}, args...)
		ls.b.Emit(&hir.Call{
			Dst:  dst,
			Fn:   "dict_new_in",
			Args: allArgs,
			Type: "ptr",
		})
	} else {
		ls.b.Emit(&hir.Call{
			Dst:  dst,
			Fn:   "dict_new",
			Args: args,
			Type: "ptr",
		})
	}
}
