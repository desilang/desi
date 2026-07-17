package lower

import (
	"bytes"
	"reflect"
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
		if set, ok := info.Funcs[fd.Name.Name]; ok && len(set.Cands) > 0 {
			funcType := set.Cands[0].Type
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
		if set, ok := info.Funcs[fd.Name.Name]; ok && len(set.Cands) > 0 {
			funcType := set.Cands[0].Type
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
	currentAllocArena hir.Value // The arena to direct allocations to during RHS lowering
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
func (ls *lowerState) emitTaskGroupWrapper(wrapperName, targetFnName string, numCaptures int) {
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
	RegisterTGWrapper(wrapperName, targetFnName, numCaptures)
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
						return mangleDesiName(decl.Name.Name)
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
								return mangleDesiName(decl.Name.Name)
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
	ls.emitTempDrops()
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
