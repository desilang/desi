package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
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
			// Get the qualified path for this import (e.g., "Container.Inner" for nested classes)
			qualifiedPath := local
			if info.FromItemPaths != nil {
				if qp, ok := info.FromItemPaths[local]; ok {
					qualifiedPath = qp
				}
			}
			// Get the module path from the module file
			for mpath, ex := range info.ModuleExports {
				if ex != nil && ex.Classes != nil {
					// First try qualified path (for nested classes like Container.Inner)
					if cls, ok := ex.Classes[qualifiedPath]; ok {
						isClass = true
						classType = cls
						_ = mpath // module path for debugging
						break
					}
					// Fall back to local name (for simple classes)
					if cls, ok := ex.Classes[local]; ok {
						isClass = true
						classType = cls
						_ = mpath
						break
					}
				}
			}
		}

		if isClass && classType != nil {
			// Register as SymFunc so constructor calls work
			// PopulateImportedFuncSigs will add constructor to Funcs map
			// The classType is stored in Type for static method and constant lookups
			top.Define(&Symbol{Name: local, Kind: SymFunc, Type: classType})
			continue
		}

		// Check if it's a type alias (e.g., "from http import Request")
		isTypeAlias := false
		var aliasType types.T
		if mod != nil && info.ModuleExports != nil {
			for _, ex := range info.ModuleExports {
				if ex != nil && ex.TypeAliases != nil {
					if t, ok := ex.TypeAliases[local]; ok {
						isTypeAlias = true
						aliasType = t
						break
					}
				}
			}
		}

		// Check if it's an exported global constant (e.g. "from config import MAX_RETRIES").
		// Without this the name fell through to the SymFunc case below and was
		// bound with no type at all, so reading it gave DTE0004 on a binding and
		// printed "<?>" inside an f-string.
		isGlobal := false
		var globalType types.T
		if mod != nil && info.ModuleExports != nil {
			for _, ex := range info.ModuleExports {
				if ex != nil && ex.Globals != nil {
					if t, ok := ex.Globals[local]; ok && t != nil {
						isGlobal = true
						globalType = t
						break
					}
				}
			}
		}

		switch {
		case isTypeAlias && aliasType != nil:
			// Register as SymType so resolveType scope lookup finds it
			top.Define(&Symbol{Name: local, Kind: SymType, Type: aliasType})
		case isGlobal:
			top.Define(&Symbol{Name: local, Kind: SymVar, Type: globalType})
		default:
			// Register as SymFunc for function imports
			top.Define(&Symbol{Name: local, Kind: SymFunc})
		}
	}
}

// injectGlobals scans the __top__ function for top-level let statements
// and binds them to the module scope so they're visible from all functions.
func injectGlobals(top *Scope, mod *ast.Module) []diag.Diagnostic {
	var out []diag.Diagnostic
	if top == nil || mod == nil {
		return out
	}

	// Find ALL synthetic __top__ functions (pub let creates separate ones)
	var topFns []*ast.FuncDecl
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "__top__" {
			topFns = append(topFns, fn)
		}
	}
	if len(topFns) == 0 {
		return out
	}

	// Scan ALL __top__ functions for LetStmt and bind globals
	for _, topFn := range topFns {
		if topFn.Body == nil {
			continue
		}
		for _, st := range topFn.Body.Stmts {
			ls, ok := st.(*ast.LetStmt)
			if !ok || ls.Name.Name == "" {
				continue
			}

			// Check 1: Global constants must be immutable
			if ls.Mutable {
				out = append(out, diagAt("DTE0051", ls.Name.Span, "global variable '"+ls.Name.Name+"' must be immutable (no 'mut')"))
			}

			// Check 2: Global constants should be UPPER_CASE (Warning)
			// Skip for sync module types (sync.Atomic, sync.Mutex, etc.) — they're mutable by design
			isSyncGlobal := false
			if ls.Value != nil {
				if ce, ok := ls.Value.(*ast.CallExpr); ok {
					if fe, ok := ce.Callee.(*ast.FieldExpr); ok {
						if modId, ok := fe.X.(*ast.Ident); ok && modId.Name == "sync" {
							isSyncGlobal = true
						}
					}
				}
			}
			if !isSyncGlobal && len(ls.Name.Name) > 0 {
				first := ls.Name.Name[0]
				if first >= 'a' && first <= 'z' {
					out = append(out, diagAt("DW0008", ls.Name.Span, "global constant '"+ls.Name.Name+"' should be UPPER_CASE"))
				}
			}

			// Resolve the type from annotation
			var t types.T
			if ls.Type != nil {
				t = resolveSimpleTypeName(ls.Type)
			} else if ls.Value != nil {
				// Infer type from literal value
				switch v := ls.Value.(type) {
				case *ast.IntLit:
					t = types.Int
				case *ast.FloatLit:
					t = types.Float
				case *ast.StrLit, *ast.FString:
					t = types.Str
				case *ast.BoolLit:
					t = types.Bool
				case *ast.NoneLit:
					t = types.None
				case *ast.BinaryExpr:
					// Arithmetic over literals: `let SECONDS_PER_DAY = 60 * 60 * 24`.
					// Inferring only from a bare literal left these as Any, which
					// showed up as "<?>" once the constant crossed a module boundary.
					t = resolve.ConstArithType(v)
				case *ast.UnaryExpr:
					// Handle negative numbers: -100, -3.14
					if v.Op == "-" {
						switch v.X.(type) {
						case *ast.IntLit:
							t = types.Int
						case *ast.FloatLit:
							t = types.Float
						}
					}
				case *ast.CallExpr:
					// Handle struct/class constructor calls like Point(x=0, y=0)
					// We need to look up the struct type from the callee name
					if id, ok := v.Callee.(*ast.Ident); ok {
						// Look up struct type by name in module declarations
						for _, d := range mod.Decls {
							if sd, ok := d.(*ast.StructDecl); ok && sd.Name.Name == id.Name {
								// Build struct type from declaration
								var fields []types.Field
								for _, f := range sd.Fields {
									var ft types.T
									if f.Type != nil {
										ft = resolveSimpleTypeName(f.Type)
									}
									fields = append(fields, types.Field{Name: f.Name.Name, Type: ft})
								}
								t = &types.Struct{Name: id.Name, Fields: fields}
								break
							}
						}
					}
					// Module-qualified constructors: sync.Atomic(0), sync.Mutex(v), etc.
					if fe, ok := v.Callee.(*ast.FieldExpr); ok {
						if modId, ok := fe.X.(*ast.Ident); ok && modId.Name == "sync" {
							switch fe.Name.Name {
							case "Atomic":
								t = types.AtomicOf()
							case "Mutex":
								t = types.MutexOf(types.Any)
							case "RWLock":
								t = types.RwLockOf(types.Any)
							case "Channel":
								t = types.ChannelOf(types.Any)
							case "Semaphore":
								t = types.SemaphoreOf()
							}
						}
					}
				}
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
	return out
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
