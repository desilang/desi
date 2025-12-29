package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

// injectImports places resolver-bound names into the top scope
// so imported identifiers behave like locals during type checking.
func injectImports(top *Scope, info *resolve.Info) {
	if info == nil || top == nil {
		return
	}
	// `import a.b [as x]`  -> bind local name as a value (module handle); not callable.
	for local := range info.Imports {
		top.Define(&Symbol{Name: local, Kind: SymVar})
	}
	// `from a.b import y [as z]` -> bind local name.
	// Check if it's a class (SymType) or function (SymFunc).
	for local, mod := range info.FromItems {
		// Try to find if this is a class in any of the module exports
		isClass := false
		var classType *types.Class
		if mod != nil && info.ModuleExports != nil {
			// Get the module path from the module file
			for mpath, ex := range info.ModuleExports {
				if ex != nil && ex.Classes != nil {
					if cls, ok := ex.Classes[local]; ok {
						isClass = true
						classType = cls
						_ = mpath // module path for debugging
						break
					}
				}
			}
		}

		if isClass && classType != nil {
			// Register as SymFunc so constructor calls work
			// PopulateImportedFuncSigs will add constructor to Funcs map
			// The classType is stored in Type for type annotations
			top.Define(&Symbol{Name: local, Kind: SymFunc, Type: classType})
		} else {
			// Register as SymFunc for function imports
			top.Define(&Symbol{Name: local, Kind: SymFunc})
		}
	}
}

// injectGlobals scans the __top__ function for top-level let statements
// and binds them to the module scope so they're visible from all functions.
func injectGlobals(top *Scope, mod *ast.Module) {
	if top == nil || mod == nil {
		return
	}

	// Find the synthetic __top__ function
	var topFn *ast.FuncDecl
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "__top__" {
			topFn = fn
			break
		}
	}
	if topFn == nil || topFn.Body == nil {
		return
	}

	// Scan for LetStmt and bind globals
	for _, st := range topFn.Body.Stmts {
		ls, ok := st.(*ast.LetStmt)
		if !ok || ls.Name.Name == "" {
			continue
		}

		// Resolve the type from annotation
		var t types.T
		if ls.Type != nil {
			t = resolveSimpleTypeName(ls.Type)
		}

		// Define the global in the top scope
		sym := &Symbol{
			Name: ls.Name.Name,
			Kind: SymVar,
			Type: t,
		}
		top.Define(sym)
	}
}

// resolveSimpleTypeName resolves basic type names for globals.
// This is a simplified version for const declarations.
func resolveSimpleTypeName(tn *ast.TypeName) types.T {
	if tn == nil {
		return nil
	}
	switch tn.Name {
	case "int":
		return types.Int
	case "float":
		return types.Float
	case "bool":
		return types.Bool
	case "str":
		return types.Str
	case "none":
		return types.None
	default:
		return nil
	}
}
