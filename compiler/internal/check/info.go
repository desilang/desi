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
	Name string
	Kind SymbolKind
	Type types.T
	Node ast.Node
}

// Info carries type facts, bindings, and overload sets discovered by the checker.
type Info struct {
	Types  map[ast.Node]types.T    // inferred types for important nodes
	Idents map[*ast.Ident]*Symbol  // bound identifiers
	Funcs  map[string]*OverloadSet // function overload sets by name

	// M5: imports bridge
	ImportPaths map[string]string // local import binding -> dotted module path (e.g., "math" -> "math")
	R           *resolve.Info     // resolver results (exports table, etc.)

	// M6-P2-B: per-function move tracking for identifiers.
	Moved map[string]diag.Span

	// M14: trait implementations (TypeName -> Trait -> []*FuncDecl)
	Impls map[string]map[string][]*ast.FuncDecl

	// Match expression: pattern variable bindings
	// Key: MatchExpr node, Value: map of arm index -> bindings
	MatchBindings map[*ast.MatchExpr]map[int][]MatchBinding

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
}

// MatchBinding represents a variable bound in a match pattern
type MatchBinding struct {
	Name       string     // Variable name (e.g., "x")
	Type       types.T    // Payload field type
	FieldIndex int        // Index in variant.Fields
	Node       *ast.Ident // The identifier node in the pattern
}

// FuncCand represents a single callable candidate.
type FuncCand struct {
	Decl       *ast.FuncDecl   // may be nil (e.g., builtins or cross-module exports)
	Type       *types.Func     // canonical function type (params + ret)
	Modes      []ast.ParamMode // callee-declared parameter modes (index-aligned with Type.Params)
	Extern     bool            // true if this candidate represents an @extern declaration
	ParamNames []string        // E-2: parameter names by index (len == arity), may be nil/empty
	Defaults   []bool          // M14: param has default value (index-aligned with Type.Params)
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
		R:                   nil,
		Moved:               make(map[string]diag.Span),
		Impls:               make(map[string]map[string][]*ast.FuncDecl),
		MatchBindings:       make(map[*ast.MatchExpr]map[int][]MatchBinding),
		FuncMoves:           make(map[*ast.FuncDecl]map[string]bool),
		ClassInstantiations: make(map[string][]*types.Generic),
		BinOpOverloads:      make(map[*ast.BinaryExpr]*FuncCand),
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
	// print(value: T) -> none
	for _, k := range coreKinds {
		add1("print", k, types.None, "value", ast.ParamMove)
	}
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
}
