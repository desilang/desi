package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// SymbolKind classifies a bound name.
type SymbolKind int

const (
	SymVar SymbolKind = iota
	SymParam
	SymFunc
	SymType // class/struct/enum/type names (placeholder in M4/M5)
)

// Symbol is a bound name with an optional static type and source association.
type Symbol struct {
	Name string
	Kind SymbolKind
	Type types.T // optional for funcs until inferred/annotated
	Node ast.Node
}

// OverloadSet groups same-named functions for exact-match resolution.
type OverloadSet struct {
	Name  string
	Cands []*FuncCand
}

// FuncCand represents one concrete function candidate (builtin or user-declared).
type FuncCand struct {
	Decl *ast.FuncDecl // may be nil (e.g., builtins)
	Type *types.Func   // canonical function type (params + ret)
}

// Info stores inference results and binding maps for a module.
type Info struct {
	Types  map[ast.Node]types.T    // inferred types for important nodes
	Idents map[*ast.Ident]*Symbol  // bound identifiers
	Funcs  map[string]*OverloadSet // function overload sets by name
}

// NewInfo returns a fresh Info and injects prelude builtins.
func NewInfo() *Info {
	info := &Info{
		Types:  make(map[ast.Node]types.T),
		Idents: make(map[*ast.Ident]*Symbol),
		Funcs:  make(map[string]*OverloadSet),
	}
	addPreludeBuiltins(info)
	return info
}

// addPreludeBuiltins installs small, exact-match overload sets for phase M4/M5.
// Keep this tiny: no variadics, no formatting, just simple exact signatures.
func addPreludeBuiltins(info *Info) {
	if info == nil {
		return
	}

	addOverloads := func(name string, params []types.T, ret types.T) {
		set := info.Funcs[name]
		if set == nil {
			set = &OverloadSet{Name: name}
			info.Funcs[name] = set
		}
		for _, p := range params {
			set.Cands = append(set.Cands, &FuncCand{
				Decl: nil,
				Type: types.FuncOf([]types.T{p}, ret),
			})
		}
	}

	core := []types.T{types.Int, types.Float, types.Bool, types.Str}
	addOverloads("print", core, types.None)
	addOverloads("str", core, types.Str)
}
