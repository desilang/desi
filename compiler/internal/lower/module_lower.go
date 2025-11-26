package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
)

// LowerModuleFromSource lowers all top-level function declarations in 'mod'.
// - Synchronous def: 1 HIR func with the same name
// - Async def:       2 HIR funcs: wrapper "<name>" and poll "<name>$poll"
//
// 'src' is used for literal materialization (strings).
// NOTE: For M9B, we also skip functions decorated with @extern(...), even if a body is present.
func LowerModuleFromSource(mod *ast.Module, info *check.Info, src []byte) *hir.Module {
	// Rewrite async lambdas into hidden async functions before lowering.
	DesugarAsyncLambdas(mod)

	out := &hir.Module{Name: mod.File}
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Skip prototypes (no body) and any function decorated with @extern(...).
		// This lets extern decls survive parsing/resolution but produce no HIR.
		if fd.Body == nil || isExternDecorated(fd) {
			continue
		}

		if fd.Async {
			w, p := LowerAsyncFunc(fd, src, info)
			if w == nil || p == nil {
				// Barrier or other early-abort: skip this function, continue module.
				continue
			}
			out.Funcs = append(out.Funcs, w, p)
			continue
		}
		out.Funcs = append(out.Funcs, LowerFuncFromDecl(fd, info, src))
	}

	// Generate constructors for struct declarations
	for _, d := range mod.Decls {
		if sd, ok := d.(*ast.StructDecl); ok {
			constructor := LowerStructConstructor(sd, info)
			if constructor != nil {
				out.Funcs = append(out.Funcs, constructor)
			}
		}
	}

	// Lower explicit ImplDecl methods
	for _, d := range mod.Decls {
		if impl, ok := d.(*ast.ImplDecl); ok {
			typeName := impl.ForType.Name
			for _, m := range impl.Methods {
				// Mangle: Type_Method
				name := fmt.Sprintf("%s_%s", typeName, m.Name.Name)
				out.Funcs = append(out.Funcs, LowerBlockFromSource(name, m.Body, info, src))
			}
		}
	}

	// Lower synthesized default Display methods
	if info != nil {
		for _, d := range mod.Decls {
			var typeName string
			if s, ok := d.(*ast.StructDecl); ok {
				typeName = s.Name.Name
			} else if c, ok := d.(*ast.ClassDecl); ok {
				typeName = c.Name.Name
			}

			if typeName != "" {
				if impls, ok := info.Impls[typeName]; ok {
					if methods, ok := impls["Display"]; ok {
						for _, m := range methods {
							// If body is nil, it's the synthesized default
							if m.Name.Name == "to_str" && m.Body == nil {
								name := fmt.Sprintf("%s_to_str", typeName)
								out.Funcs = append(out.Funcs, LowerDefaultToStr(name, typeName))
							}
						}
					}
				}
			}
		}
	}

	return out
}

// isExternDecorated reports whether the function has an @extern(...) decorator.
func isExternDecorated(fd *ast.FuncDecl) bool {
	for _, dec := range fd.Decorators {
		if dec.Name.Name == "extern" {
			return true
		}
	}
	return false
}

// LowerStructConstructor generates a constructor function for a struct.
// Constructor signature: StructName(field1_type %field1, ...) -> ptr
// Implementation: alloca struct, store fields, return pointer
func LowerStructConstructor(sd *ast.StructDecl, info *check.Info) *hir.Func {
	structName := sd.Name.Name
	b := hir.NewFunc(structName)

	// Create parameters from struct fields
	var params []hir.Param
	for _, field := range sd.Fields {
		fieldType := "i32" // default to int
		if field.Type != nil {
			// Map type name to LLVM type
			switch field.Type.Name {
			case "int", "i32":
				fieldType = "i32"
			case "i64", "u64":
				fieldType = "i64"
			case "bool":
				fieldType = "i1"
			case "str":
				fieldType = "ptr"
			default:
				fieldType = "i32" // default
			}
		}
		params = append(params, hir.Param{
			Name: field.Name.Name,
			Type: fieldType,
		})
	}

	// Calculate struct size (simplified - assume all fields are same size for now)
	// In real implementation, would need proper struct layout
	structSize := len(sd.Fields) * 8 // 8 bytes per field (i64/ptr)

	// Create entry block
	entry := hir.NewBlock("entry")

	// Allocate space for struct
	structPtr := hir.Temp{Name: "%struct_ptr"}
	entry.Stmts = append(entry.Stmts, &hir.Alloca{
		Type:  "i8", // byte array
		Count: structSize,
		Dst:   structPtr,
	})

	// Store each field
	for i, field := range sd.Fields {
		paramVar := hir.Var{Name: field.Name.Name}

		// GEP to field offset
		fieldPtr := hir.Temp{Name: fmt.Sprintf("%%field_%s_ptr", field.Name.Name)}
		entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
			Type:    "i8",
			Base:    structPtr,
			Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", i*8)}}, // Simplified offset
			Dst:     fieldPtr,
		})

		// Store parameter value to field
		entry.Stmts = append(entry.Stmts, &hir.Store{
			Dst: fieldPtr,
			Val: paramVar,
		})
	}

	// Return struct pointer
	entry.Stmts = append(entry.Stmts, &hir.Ret{Val: structPtr})

	f := b.Func()
	f.Name = structName
	f.Params = params
	f.RetType = "ptr" // Return pointer to struct
	f.Blocks = []*hir.Block{entry}

	return f
}
