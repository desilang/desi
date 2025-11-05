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

// Symbol represents a bound identifier.
type Symbol struct {
	Name string
	Kind SymbolKind
	Type *types.Func // optional, only used for function symbols in some checks
	Node ast.Node
}

// Info holds checker state and cross-phase bridges.
type Info struct {
	Types  map[ast.Node]types.T    // inferred types for important nodes
	Idents map[*ast.Ident]*Symbol  // bound identifiers
	Funcs  map[string]*OverloadSet // function overload sets by name

	// M5: imports bridge
	ImportPaths map[string]string // local import binding -> dotted module path (e.g., "math" -> "math")
	R           *resolve.Info     // resolver results (exports table, etc.)

	// M6-P2-B: per-function move tracking for identifiers.
	Moved map[string]diag.Span

	// M9C: current unsafe nesting depth while walking.
	UnsafeDepth int
}

// FuncCand represents a single callable candidate.
type FuncCand struct {
	Decl   *ast.FuncDecl   // may be nil (e.g., builtins or cross-module exports)
	Type   *types.Func     // canonical function type (params + ret)
	Modes  []ast.ParamMode // callee-declared parameter modes (index-aligned with Type.Params)
	Extern bool            // M9C: whether this candidate is an extern (@extern) declaration
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
	addOverloads := func(name string, params []types.T, ret types.T) {
		set, ok := info.Funcs[name]
		if !ok || set == nil {
			set = &OverloadSet{Name: name}
			info.Funcs[name] = set
		}
		for _, p := range params {
			set.Cands = append(set.Cands, &FuncCand{
				Decl:   nil,
				Type:   types.FuncOf([]types.T{p}, ret),
				Modes:  []ast.ParamMode{ast.ParamMove}, // builtins: treat as move-by-value
				Extern: false,
			})
		}
	}
	core := []types.T{types.Int, types.Float, types.Bool, types.Str}
	addOverloads("print", core, types.None) // print(x) -> none
	addOverloads("str", core, types.Str)    // str(x) -> str
}
