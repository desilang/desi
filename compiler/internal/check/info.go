package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

// SymbolKind classifies a bound name.
type SymbolKind int

const (
	SymVar SymbolKind = iota
	SymParam
	SymFunc
	SymType // M14: type name
)

// Symbol represents a bound identifier in a scope.
type Symbol struct {
	Name      string
	Kind      SymbolKind
	Type      types.T
	Node      ast.Node
	IsMutable bool // true for `let mut`, false for `let`
}

// Info carries type facts, bindings, and overload sets discovered by the checker.
type Info struct {
	Types  map[ast.Node]types.T    // inferred types for important nodes
	Idents map[*ast.Ident]*Symbol  // bound identifiers
	Funcs  map[string]*OverloadSet // function overload sets by name

	// M5: imports bridge
	ImportPaths    map[string]string   // local import binding -> dotted module path (e.g., "math" -> "math")
	ImportAliases  map[string]string   // local alias name -> actual function name (e.g., "sum" -> "add")
	LambdaAliases  map[string]string   // lambda variable name -> synthesized hidden func name (e.g., "double" -> "__lam$0")
	LambdaCaptures map[string][]string // synthesized func name -> captured variable names (e.g., "__lam$0" -> ["offset"])
	R              *resolve.Info       // resolver results (exports table, etc.)

	// M6-P2-B: per-function move tracking for identifiers.
	Moved map[string]diag.Span

	// M14: trait implementations (TypeName -> Trait -> []*FuncDecl)
	Impls map[string]map[string][]*ast.FuncDecl

	// Match expression: pattern variable bindings
	// Key: MatchExpr node, Value: map of arm index -> bindings
	MatchBindings map[*ast.MatchExpr]map[int][]MatchBinding

	// Is expression: pattern variable bindings
	// Key: IsExpr node, Value: list of bindings (for patterns like `is Some(val)`)
	IsBindings map[*ast.IsExpr][]MatchBinding

	// FuncMoves tracks which variables are moved in each function.
	// Key: FuncDecl, Value: Set of moved variable names.
	// This is used by the backend to avoid double-freeing moved variables.
	FuncMoves map[*ast.FuncDecl]map[string]bool

	// ClassInstantiations tracks generic class instantiations for monomorphization.
	// Key: base class name, Value: list of instantiated Generic types
	ClassInstantiations map[string][]*types.Generic

	// BinOpOverloads maps binary expressions to their resolved operator method candidate.
	// This allows the backend to emit a method call instead of a primitive binary op.
	BinOpOverloads map[*ast.BinaryExpr]*FuncCand

	// TestFuncs tracks functions decorated with @test for the test runner.
	// Key: function name, Value: the FuncDecl node
	TestFuncs map[string]*ast.FuncDecl

	// StdlibImports tracks which stdlib modules have been imported.
	// Key: module name (e.g., "log", "json"), Value: true if imported
	// Used for dead-code elimination and validation of stdlib usage.
	StdlibImports map[string]bool

	// BoolConversions tracks expressions that need __bool__ dunder calls.
	// Key: expression node (condition in if/while), Value: the class type with __bool__
	// The lowering phase uses this to emit a method call instead of using the value directly.
	BoolConversions map[ast.Expr]*types.Class

	// DbgCalls tracks dbg() calls with their source location and expression text.
	// Key: the CallExpr node for dbg(), Value: debug info for emission
	DbgCalls map[*ast.CallExpr]*DbgCallInfo

	// ChosenOverloads tracks which overload was selected for each call site.
	// Key: the CallExpr node, Value: the chosen FuncCand.
	// Used by the lowerer to emit the correct C symbol for overloaded functions.
	ChosenOverloads map[*ast.CallExpr]*FuncCand

	// NonEscaping tracks local variables that do not escape their containing function.
	// Key: local variable AST Ident node, Value: true if safe for arena allocation.
	NonEscaping map[*ast.Ident]bool

	// PerfLevel is the configured performance advisor level ("relaxed", "default", "strict").
	// Set by CLI flags (e.g., desic perf --level=strict) and read by RunPerfAdvisor.
	// Empty string means "default".
	PerfLevel string
}

// DbgCallInfo stores metadata for a dbg() call to emit [file:line] expr = value
type DbgCallInfo struct {
	File     string // Source file name
	Line     int    // Line number
	ExprText string // The source text of the expression (e.g., "x + 1")
}

// MatchBinding represents a variable bound in a match pattern
type MatchBinding struct {
	Name          string        // Variable name (e.g., "x")
	Type          types.T       // Payload field type
	FieldIndex    int           // Index in variant.Fields (-1 for scrutinee binding)
	Node          *ast.Ident    // The identifier node in the pattern
	NestedPattern *ast.CallExpr // Nested pattern (e.g., Some(v) in Some(Some(v)))
	NestedType    types.T       // Type of the nested pattern value
}

// FuncCand represents a single callable candidate.
type FuncCand struct {
	Decl       *ast.FuncDecl   // may be nil (e.g., builtins or cross-module exports)
	Type       *types.Func     // canonical function type (params + ret)
	Modes      []ast.ParamMode // callee-declared parameter modes (index-aligned with Type.Params)
	Extern     bool            // true if this candidate represents an @extern declaration
	ParamNames []string        // E-2: parameter names by index (len == arity), may be nil/empty
	Defaults   []bool          // M14: param has default value (index-aligned with Type.Params)
	ModuleDecl *ast.FuncDecl   // module pub def FuncDecl (for lowerer overload dispatch only)
}

// OverloadSet groups candidate functions by name.
type OverloadSet struct {
	Name  string
	Cands []*FuncCand
}

// NewInfo allocates a fresh Info and pre-populates prelude builtins.
func NewInfo() *Info {
	info := &Info{
		Types:               make(map[ast.Node]types.T),
		Idents:              make(map[*ast.Ident]*Symbol),
		Funcs:               make(map[string]*OverloadSet),
		ImportPaths:         make(map[string]string),
		ImportAliases:       make(map[string]string),
		R:                   nil,
		Moved:               make(map[string]diag.Span),
		Impls:               make(map[string]map[string][]*ast.FuncDecl),
		MatchBindings:       make(map[*ast.MatchExpr]map[int][]MatchBinding),
		FuncMoves:           make(map[*ast.FuncDecl]map[string]bool),
		ClassInstantiations: make(map[string][]*types.Generic),
		BinOpOverloads:      make(map[*ast.BinaryExpr]*FuncCand),
		TestFuncs:           make(map[string]*ast.FuncDecl),
		StdlibImports:       make(map[string]bool),
		BoolConversions:     make(map[ast.Expr]*types.Class),
		DbgCalls:            make(map[*ast.CallExpr]*DbgCallInfo),
		ChosenOverloads:     make(map[*ast.CallExpr]*FuncCand),
		NonEscaping:         make(map[*ast.Ident]bool),
	}
	addPreludeBuiltins(info)
	return info
}

// addPreludeBuiltins seeds overloads for a few core builtins used in tests.
func addPreludeBuiltins(info *Info) {
	if info == nil {
		return
	}

	// Helpers that attach ParamNames so named-args work on builtins.
	makeNames := func(n int, names ...string) []string {
		out := make([]string, n)
		copy(out, names)
		return out
	}
	add1 := func(name string, param types.T, ret types.T, pname string, mode ast.ParamMode) {
		set := info.Funcs[name]
		if set == nil {
			set = &OverloadSet{Name: name}
			info.Funcs[name] = set
		}
		set.Add(&FuncCand{
			Decl:       nil,
			Type:       types.FuncOf([]types.T{param}, ret, false),
			Modes:      []ast.ParamMode{mode},
			ParamNames: makeNames(1, pname),
			Defaults:   nil, // builtins have no defaults in M14
		})
	}
	addN := func(name string, params []types.T, modes []ast.ParamMode, ret types.T, pnames []string) {
		set := info.Funcs[name]
		if set == nil {
			set = &OverloadSet{Name: name}
			info.Funcs[name] = set
		}
		set.Add(&FuncCand{
			Decl:       nil,
			Type:       types.FuncOf(params, ret, false),
			Modes:      modes,
			ParamNames: makeNames(len(params), pnames...),
			Defaults:   nil, // builtins have no defaults in M14
		})
	}

	// Canonical kinds to avoid ambiguity.
	coreKinds := []types.T{types.Int, types.Float, types.Bool, types.Str}

	// --- Task A builtins (unary) ---
	// print(*args: Any) -> none - variadic print for any number of arguments
	// Uses list[Any] as the variadic parameter type
	info.Funcs["print"] = &OverloadSet{Name: "print"}
	info.Funcs["print"].Add(&FuncCand{
		Type: &types.Func{
			Params:   []types.T{types.ListOf(types.Any)},
			Ret:      types.None,
			Variadic: true,
		},
		ParamNames: []string{"args"},
		Modes:      []ast.ParamMode{ast.ParamMove},
		Defaults:   []bool{true}, // variadic params are implicitly optional
	})
	// str(value: T) -> str
	for _, k := range coreKinds {
		add1("str", k, types.Str, "value", ast.ParamMove)
	}
	// bool(value: T) -> bool
	for _, k := range coreKinds {
		add1("bool", k, types.Bool, "value", ast.ParamMove)
	}
	// len(s: str) -> usize
	add1("len", types.Str, types.USize, "s", ast.ParamMove)

	// dbg(value: Any) -> Any - debug print with file:line and expression text
	// Returns the value unchanged (pass-through)
	info.Funcs["dbg"] = &OverloadSet{Name: "dbg"}
	info.Funcs["dbg"].Add(&FuncCand{
		Type: &types.Func{
			Params:   []types.T{types.Any},
			Ret:      types.Any,
			Variadic: false,
		},
		ParamNames: []string{"value"},
		Modes:      []ast.ParamMode{ast.ParamMove},
	})

	// type_of(value: Any) -> Type[T] - runtime type introspection
	// Returns a Type[T] wrapper containing the value's type information
	// Special handling in expr_call.go to return Type[T] where T is the arg's type
	info.Funcs["type_of"] = &OverloadSet{Name: "type_of"}
	info.Funcs["type_of"].Add(&FuncCand{
		Type: &types.Func{
			Params:   []types.T{types.Any},
			Ret:      types.Any, // replaced with Type[T] in expr_call.go
			Variadic: false,
		},
		ParamNames: []string{"value"},
		Modes:      []ast.ParamMode{ast.ParamMove},
	})

	// --- Task D stubs (compile-only, with explicit names) ---
	// list_push(list: _, value: int) -> none
	addN("list_push",
		[]types.T{nil, types.Int},
		[]ast.ParamMode{ast.ParamInout, ast.ParamMove},
		types.None,
		[]string{"list", "value"},
	)
	// set_add(set: _, value: str) -> none
	addN("set_add",
		[]types.T{nil, types.Str},
		[]ast.ParamMode{ast.ParamInout, ast.ParamMove},
		types.None,
		[]string{"set", "value"},
	)
	// dict_set(dict: _, key: str, value: int) -> none
	addN("dict_set",
		[]types.T{nil, types.Str, types.Int},
		[]ast.ParamMode{ast.ParamInout, ast.ParamMove, ast.ParamMove},
		types.None,
		[]string{"dict", "key", "value"},
	)

	// --- Task E: range surface (typed params; opaque return for now) ---
	// range(stop: int) -> _
	addN("range",
		[]types.T{types.Int},
		[]ast.ParamMode{ast.ParamMove},
		nil,
		[]string{"stop"},
	)
	// range(start: int, stop: int) -> _
	addN("range",
		[]types.T{types.Int, types.Int},
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
		nil,
		[]string{"start", "stop"},
	)
	// range(start: int, stop: int, step: int) -> _
	addN("range",
		[]types.T{types.Int, types.Int, types.Int},
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove, ast.ParamMove},
		nil,
		[]string{"start", "stop", "step"},
	)

	// --- enumerate builtin ---
	// enumerate(iterable) -> iterator of (index, element) pairs
	// The actual type checking for enumerate is handled specially in stmt.go
	// since it produces two loop variables
	addN("enumerate",
		[]types.T{nil}, // Any iterable type
		[]ast.ParamMode{ast.ParamMove},
		nil, // Special return handled by for-loop
		[]string{"iterable"},
	)

	// --- reversed builtin ---
	// reversed(iterable) -> iterator in reverse order
	// Handled specially in stmt.go for for-loop lowering
	addN("reversed",
		[]types.T{nil}, // Any iterable type
		[]ast.ParamMode{ast.ParamMove},
		nil, // Special return handled by for-loop
		[]string{"iterable"},
	)

	// --- sum builtin ---
	// sum(items: list[int]) -> int
	// Special handling in expr_call.go
	addN("sum",
		[]types.T{nil}, // list[int] or list[float]
		[]ast.ParamMode{ast.ParamMove},
		nil, // int or float depending on input
		[]string{"items"},
	)

	// --- min/max builtins ---
	// min(items: list[int]) -> int
	// max(items: list[int]) -> int
	// Special handling in expr_call.go
	addN("min",
		[]types.T{nil},
		[]ast.ParamMode{ast.ParamMove},
		nil,
		[]string{"items"},
	)
	addN("max",
		[]types.T{nil},
		[]ast.ParamMode{ast.ParamMove},
		nil,
		[]string{"items"},
	)

	// --- any/all builtins ---
	// any(items: list[bool]) -> bool
	// all(items: list[bool]) -> bool
	// Special handling in expr_call.go
	addN("any",
		[]types.T{nil},
		[]ast.ParamMode{ast.ParamMove},
		types.Bool,
		[]string{"items"},
	)
	addN("all",
		[]types.T{nil},
		[]ast.ParamMode{ast.ParamMove},
		types.Bool,
		[]string{"items"},
	)

	// --- sorted builtin ---
	// sorted(items: list[T]) -> list[T]
	// sorted(items: list[T], reverse: bool) -> list[T]
	// Special handling in expr_call.go, use Any for pipe compatibility
	addN("sorted",
		[]types.T{types.Any}, // accepts any list
		[]ast.ParamMode{ast.ParamMove},
		types.Any, // return type determined in expr_call.go
		[]string{"items"},
	)
	addN("sorted",
		[]types.T{types.Any, types.Bool}, // accepts any list, bool
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
		types.Any, // return type determined in expr_call.go
		[]string{"items", "reverse"},
	)

	// --- zip builtin ---
	// zip(a, b) -> iterator of (a[i], b[i]) pairs
	// Special handling in stmt.go for for-loop lowering
	addN("zip",
		[]types.T{nil, nil}, // Two iterables
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
		nil, // Special return handled by for-loop
		[]string{"a", "b"},
	)

	// --- File I/O builtins ---
	// open(path: str, mode: str) -> File
	addN("open",
		[]types.T{types.Str, types.Str},
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
		types.File,
		[]string{"path", "mode"},
	)

	// --- Testing builtins ---
	// assert(condition: bool) -> none
	add1("assert", types.Bool, types.None, "condition", ast.ParamMove)
	// assert(condition: bool, message: str) -> none
	addN("assert",
		[]types.T{types.Bool, types.Str},
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
		types.None,
		[]string{"condition", "message"},
	)

	// assert_eq(expected: T, actual: T) -> none  — for int, str, bool
	for _, k := range []types.T{types.Int, types.Str, types.Bool} {
		addN("assert_eq",
			[]types.T{k, k},
			[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
			types.None,
			[]string{"expected", "actual"},
		)
	}
	// assert_eq(expected: T, actual: T, message: str) -> none
	for _, k := range []types.T{types.Int, types.Str, types.Bool} {
		addN("assert_eq",
			[]types.T{k, k, types.Str},
			[]ast.ParamMode{ast.ParamMove, ast.ParamMove, ast.ParamMove},
			types.None,
			[]string{"expected", "actual", "message"},
		)
	}

	// assert_ne(a: T, b: T) -> none  — for int, str, bool
	for _, k := range []types.T{types.Int, types.Str, types.Bool} {
		addN("assert_ne",
			[]types.T{k, k},
			[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
			types.None,
			[]string{"a", "b"},
		)
	}
	// assert_ne(a: T, b: T, message: str) -> none
	for _, k := range []types.T{types.Int, types.Str, types.Bool} {
		addN("assert_ne",
			[]types.T{k, k, types.Str},
			[]ast.ParamMode{ast.ParamMove, ast.ParamMove, ast.ParamMove},
			types.None,
			[]string{"a", "b", "message"},
		)
	}

	// --- reduce/fold builtins ---
	// reduce(func, iterable, initial) -> AccT  (left-to-right fold)
	// foldl(func, iterable, initial) -> AccT   (alias for reduce)
	// foldr(func, iterable, initial) -> AccT   (right-to-left fold)
	// Special handling in expr_call.go for type checking
	addN("reduce",
		[]types.T{nil, nil, nil}, // func, iterable, initial
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove, ast.ParamMove},
		nil, // AccT - determined by initial value in expr_call.go
		[]string{"func", "iterable", "initial"},
	)
	addN("foldl",
		[]types.T{nil, nil, nil},
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove, ast.ParamMove},
		nil,
		[]string{"func", "iterable", "initial"},
	)
	addN("foldr",
		[]types.T{nil, nil, nil},
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove, ast.ParamMove},
		nil,
		[]string{"func", "iterable", "initial"},
	)

	// --- mutex_new builtin ---
	// mutex_new(value: T) -> Mutex[T]
	// Special handling in expr_call.go for type inference
	addN("mutex_new",
		[]types.T{nil}, // Any type - Mutex[T] inferred from argument
		[]ast.ParamMode{ast.ParamMove},
		nil, // Mutex[T] - determined in expr_call.go
		[]string{"value"},
	)

	// --- channel_new builtin ---
	// channel_new(capacity: int) -> Channel[T]
	// Note: Element type T must be specified via type annotation on the variable
	// e.g., let ch: Channel[int] = channel_new(10)
	addN("channel_new",
		[]types.T{types.Int}, // Buffer capacity
		[]ast.ParamMode{ast.ParamMove},
		nil, // Channel[T] - requires type annotation
		[]string{"capacity"},
	)

	// --- taskgroup_new builtin ---
	// taskgroup_new() -> TaskGroup
	addN("taskgroup_new",
		[]types.T{}, // No arguments
		[]ast.ParamMode{},
		types.TaskGroupOf(),
		[]string{},
	)

	// --- rc/arc builtins for reference counting ---
	// rc(value: T) -> Rc[T]  (single-threaded refcount)
	// arc(value: T) -> Arc[T] (thread-safe refcount)
	// Special handling in expr_call.go for type inference
	addN("rc",
		[]types.T{nil}, // Any type - Rc[T] inferred from argument
		[]ast.ParamMode{ast.ParamMove},
		nil, // Rc[T] - determined in expr_call.go
		[]string{"value"},
	)
	addN("arc",
		[]types.T{nil}, // Any type - Arc[T] inferred from argument
		[]ast.ParamMode{ast.ParamMove},
		nil, // Arc[T] - determined in expr_call.go
		[]string{"value"},
	)

	// --- Runtime configuration builtins ---
	// set_recursion_limit(limit: int) -> none
	// Like Python's sys.setrecursionlimit(n), sets the max call depth
	add1("set_recursion_limit", types.Int, types.None, "limit", ast.ParamMove)

	// --- Hot reload state serialization builtins ---
	// is_reload() -> bool: Returns true if this is a hot reload (not first run)
	addN("is_reload", nil, nil, types.Bool, nil)
	// reload_count() -> int: Returns how many times we've reloaded (0 = first run)
	addN("reload_count", nil, nil, types.Int, nil)
	// state_file() -> Option[str]: Returns the state file path if available
	addN("state_file", nil, nil, types.OptionOf(types.Str), nil)
	// write_state(json: str) -> bool: Write JSON string to state file
	add1("write_state", types.Str, types.Bool, "json", ast.ParamMove)
	// read_state() -> Option[str]: Read JSON string from state file
	addN("read_state", nil, nil, types.OptionOf(types.Str), nil)
	// delete_state() -> bool: Delete state file after successful restore
	addN("delete_state", nil, nil, types.Bool, nil)

	// --- chr/ord builtins ---
	// chr(n: int) -> str — Unicode codepoint to single-char string
	add1("chr", types.Int, types.Str, "n", ast.ParamMove)
	// ord(s: str) -> int — First char of string to Unicode codepoint
	add1("ord", types.Str, types.Int, "s", ast.ParamMove)

	// --- hex/oct/bin builtins ---
	// hex(n: int) -> str — Integer to hex string with 0x prefix
	add1("hex", types.Int, types.Str, "n", ast.ParamMove)
	// oct(n: int) -> str — Integer to octal string with 0o prefix
	add1("oct", types.Int, types.Str, "n", ast.ParamMove)
	// bin(n: int) -> str — Integer to binary string with 0b prefix
	add1("bin", types.Int, types.Str, "n", ast.ParamMove)

	// NOTE: abs() and pow() are NOT registered as prelude builtins.
	// abs conflicts with @extern("C", c_name="abs") in safe FFI wrappers.
	// pow conflicts with math.pow(float, float) -> float module wrapper.
	// Both work via direct lowerer dispatch in lower_call.go.

	// --- round builtin ---
	// round(n: float) -> float (0 decimal places)
	add1("round", types.Float, types.Float, "n", ast.ParamMove)
	// round(n: float, digits: int) -> float
	addN("round",
		[]types.T{types.Float, types.Int},
		[]ast.ParamMode{ast.ParamMove, ast.ParamMove},
		types.Float,
		[]string{"n", "digits"},
	)

	// NOTE: pow() is NOT registered as a prelude builtin to avoid
	// conflicting with math.pow(float, float) -> float. The standalone
	// pow(int, int) -> int lowerer dispatch still works for direct calls.



	// --- todo builtin (Rust-inspired) ---
	// todo() -> never — panics with "not implemented"
	addN("todo", nil, nil, types.None, nil)
	// todo(msg: str) -> never — panics with custom message
	add1("todo", types.Str, types.None, "msg", ast.ParamMove)

	// --- unreachable builtin (Rust-inspired) ---
	// unreachable() -> never — panics if an ostensibly unreachable code path is hit
	addN("unreachable", nil, nil, types.None, nil)

	// --- hash builtin ---
	// hash(value: Any) -> int — hash of a value
	add1("hash", types.Any, types.Int, "value", ast.ParamMove)

	// --- id builtin ---
	// id(value: Any) -> int — pointer identity
	add1("id", types.Any, types.Int, "value", ast.ParamMove)
}
