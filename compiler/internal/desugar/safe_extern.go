// Package desugar transforms syntactic sugar into core AST before type checking.
package desugar

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// ExpandSafeExterns transforms @extern("C", safe=true, c_name="...") declarations
// into a hidden raw extern and a public safe wrapper.
//
// Input:
//
//	@extern("C", safe=true, c_name="localtime_r")
//	pub def localtime(timestamp: i64) -> TmStruct
//
// Output:
//
//	@extern("C")
//	def __extern_localtime_r(__arg0: cptr[i64], __out: cptr[TmStruct]) -> cptr[TmStruct]
//
//	pub def localtime(timestamp: i64) -> TmStruct:
//	    unsafe:
//	        let __result: TmStruct = zeroed()
//	        __extern_localtime_r(&timestamp, &__result)
//	        return __result
func ExpandSafeExterns(mod *ast.Module) *ast.Module {
	var newDecls []ast.Decl

	for _, decl := range mod.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			newDecls = append(newDecls, decl)
			continue
		}

		// Check if this is a safe extern
		safeInfo := getSafeExternInfo(fd)
		if safeInfo == nil {
			newDecls = append(newDecls, decl)
			continue
		}

		// Generate the hidden raw extern and wrapper
		rawExtern, wrapper := generateSafeWrapper(fd, safeInfo)
		newDecls = append(newDecls, rawExtern, wrapper)
	}

	mod.Decls = newDecls
	return mod
}

// SafeExternInfo holds parsed info from @extern("C", safe=true, c_name="...")
type SafeExternInfo struct {
	CName     string   // C function name
	OutParams []string // explicit out parameter names (optional)
}

// getSafeExternInfo checks if fd has @extern with safe=true and returns parsed info.
func getSafeExternInfo(fd *ast.FuncDecl) *SafeExternInfo {
	for _, dec := range fd.Decorators {
		if dec.Name.Name != "extern" {
			continue
		}
		if dec.KwArgs == nil {
			continue
		}

		// Check safe=true
		safeExpr, hasSafe := dec.KwArgs["safe"]
		if !hasSafe {
			continue
		}
		safeBool, ok := safeExpr.(*ast.BoolLit)
		if !ok || !safeBool.Value {
			continue
		}

		// Get c_name
		cNameExpr, hasCName := dec.KwArgs["c_name"]
		if !hasCName {
			continue
		}
		cNameStr, ok := cNameExpr.(*ast.StrLit)
		if !ok {
			continue
		}

		// Value contains the lexeme which includes quotes - strip them
		cName := stripQuotes(cNameStr.Value)
		if cName == "" {
			continue
		}

		info := &SafeExternInfo{
			CName: cName,
		}

		// Optional: out=["param1", "param2"]
		if outExpr, hasOut := dec.KwArgs["out"]; hasOut {
			if outList, ok := outExpr.(*ast.ListLit); ok {
				for _, elem := range outList.Elems {
					if str, ok := elem.(*ast.StrLit); ok {
						info.OutParams = append(info.OutParams, stripQuotes(str.Value))
					}
				}
			}
		}

		return info
	}
	return nil
}

// stripQuotes removes surrounding quotes from a string literal value.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// generateSafeWrapper creates the raw extern and safe wrapper functions.
func generateSafeWrapper(fd *ast.FuncDecl, info *SafeExternInfo) (*ast.FuncDecl, *ast.FuncDecl) {
	// Raw extern uses the actual C function name so linker resolves it
	rawName := info.CName

	// Create raw extern declaration
	rawExtern := &ast.FuncDecl{
		Name: ast.Ident{Name: rawName, Span: fd.Name.Span},
		Decorators: []*ast.Decorator{
			{
				Name: ast.Ident{Name: "extern", Span: fd.Name.Span},
				Args: []ast.Expr{&ast.StrLit{Value: "C", Span: fd.Name.Span}},
				Span: fd.Name.Span,
			},
		},
		Pub:     false,
		RetType: fd.RetType, // Keep same return type for now
		Body:    nil,        // Extern has no body
		Span:    fd.Span,
	}

	// Copy params
	for _, p := range fd.Params {
		rawExtern.Params = append(rawExtern.Params, p)
	}

	// Create safe wrapper
	wrapper := &ast.FuncDecl{
		Name:    fd.Name,
		Params:  fd.Params,
		RetType: fd.RetType,
		Pub:     fd.Pub,
		Span:    fd.Span,
		// Body will contain unsafe block with call to rawExtern
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.UnsafeBlock{
					Body: &ast.Block{
						Stmts: []ast.Stmt{
							// Call raw extern and return
							// Simplified: just call the raw extern directly
							&ast.ReturnStmt{
								Value: &ast.CallExpr{
									Callee: &ast.Ident{Name: rawName, Span: fd.Name.Span},
									Args:   buildArgRefs(fd.Params),
									Span:   fd.Span,
								},
								Span: fd.Span,
							},
						},
						Span: fd.Span,
					},
					Span: fd.Span,
				},
			},
			Span: fd.Span,
		},
	}

	return rawExtern, wrapper
}

// buildArgRefs creates identifier references for each parameter.
func buildArgRefs(params []ast.Param) []ast.Expr {
	var args []ast.Expr
	for _, p := range params {
		args = append(args, &ast.Ident{Name: p.Name.Name, Span: p.Name.Span})
	}
	return args
}
