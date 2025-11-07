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
}

// FuncCand represents a single callable candidate.
type FuncCand struct {
	Decl   *ast.FuncDecl   // may be nil (e.g., builtins or cross-module exports)
	Type   *types.Func     // canonical function type (params + ret)
	Modes  []ast.ParamMode // callee-declared parameter modes (index-aligned with Type.Params)
	Extern bool            // true if this candidate represents an @extern declaration
}

// OverloadSet groups candidate functions by name.
type OverloadSet struct {
	Name  string
	Cands []*FuncCand
}

// NewInfo allocates a fresh Info and pre-populates prelude builtins.
func NewInfo() *Info {
	info := &Info{
		Types:       make(map[ast.Node]types.T),
		Idents:      make(map[*ast.Ident]*Symbol),
		Funcs:       make(map[string]*OverloadSet),
		ImportPaths: make(map[string]string),
		R:           nil,
		Moved:       make(map[string]diag.Span),
	}
	addPreludeBuiltins(info)
	return info
}

// addPreludeBuiltins seeds overloads for a few core builtins used in tests.
func addPreludeBuiltins(info *Info) {
	if info == nil {
		return
	}
	// Helper: add a family of 1-arg overloads name(T) -> ret for T in params.
	addOverloads := func(name string, params []types.T, ret types.T) {
		set := info.Funcs[name]
		if set == nil {
			set = &OverloadSet{Name: name}
			info.Funcs[name] = set
		}
		for _, p := range params {
			set.Add(&FuncCand{
				Decl:  nil,
				Type:  types.FuncOf([]types.T{p}, ret),
				Modes: []ast.ParamMode{ast.ParamMove}, // builtins: treat as move-by-value
			})
		}
	}

	// Helper: add a single overload with explicit param modes.
	addWithModes := func(name string, params []types.T, modes []ast.ParamMode, ret types.T) {
		set := info.Funcs[name]
		if set == nil {
			set = &OverloadSet{Name: name}
			info.Funcs[name] = set
		}
		set.Add(&FuncCand{
			Decl:  nil,
			Type:  types.FuncOf(params, ret),
			Modes: modes,
		})
	}

	// Canonical “kinds” only to avoid ambiguity across sized numerics.
	coreKinds := []types.T{types.Int, types.Float, types.Bool, types.Str}

	// Existing builtins from Task A:
	addOverloads("print", coreKinds, types.None)
	addOverloads("str", coreKinds, types.Str)
	addOverloads("bool", coreKinds, types.Bool)
	addOverloads("len", []types.T{types.Str}, types.USize)

	// ---- Task D: minimal collection ops (compile-only stubs) ----
	// We don’t declare concrete collection types yet; the first param type
	// is left as nil to act as “any collection” placeholder. This keeps
	// overload resolution simple (single candidate) while enforcing inout.
	addWithModes("list_push",
		[]types.T{nil, types.Int},
		[]ast.ParamMode{ast.ParamInout, ast.ParamMove},
		types.None)

	addWithModes("set_add",
		[]types.T{nil, types.Str},
		[]ast.ParamMode{ast.ParamInout, ast.ParamMove},
		types.None)

	addWithModes("dict_set",
		[]types.T{nil, types.Str, types.Int},
		[]ast.ParamMode{ast.ParamInout, ast.ParamMove, ast.ParamMove},
		types.None)
	// -------------------------------------------------------------
}
