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
	SymType // class/struct/enum/type names (placeholder in M4)
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

type FuncCand struct {
	Decl *ast.FuncDecl // may be nil (e.g., builtins)
	Type *types.Func   // canonical function type (params + ret)
}

// Info stores inference results and binding maps for a module.
type Info struct {
	Types  map[ast.Node]types.T // inferred types for important nodes
	Idents map[*ast.Ident]*Symbol
	Funcs  map[string]*OverloadSet
}

func NewInfo() *Info {
	return &Info{
		Types:  make(map[ast.Node]types.T),
		Idents: make(map[*ast.Ident]*Symbol),
		Funcs:  make(map[string]*OverloadSet),
	}
}
