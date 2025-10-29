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

// NewInfo constructs an Info and pre-populates it with prelude builtins
// that must always be available (Phase-1).
func NewInfo() *Info {
  inf := &Info{
    Types:  make(map[ast.Node]types.T),
    Idents: make(map[*ast.Ident]*Symbol),
    Funcs:  make(map[string]*OverloadSet),
  }
  addPreludeBuiltins(inf)
  return inf
}

// addPreludeBuiltins installs a minimal prelude sufficient for Phase-1 demos.
// We keep it conservative: exact-match overloads for common scalar types.
// Return type is 'none'.
func addPreludeBuiltins(info *Info) {
  if info == nil {
    return
  }
  // print(T) -> none  for T in {int, float, bool, str}
  ps := []types.T{types.Int, types.Float, types.Bool, types.Str}
  set := &OverloadSet{Name: "print"}
  for _, p := range ps {
    ft := types.FuncOf([]types.T{p}, types.None)
    set.Cands = append(set.Cands, &FuncCand{Decl: nil, Type: ft})
  }
  info.Funcs["print"] = set
}
