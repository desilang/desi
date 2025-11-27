package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
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
// Implementation: malloc struct, store fields, return pointer
func LowerStructConstructor(sd *ast.StructDecl, info *check.Info) *hir.Func {
	structName := sd.Name.Name
	b := hir.NewFunc(structName)

	// Look up struct type to get resolved field types
	var st *types.Struct
	if t := info.Types[sd]; t != nil {
		st, _ = t.(*types.Struct)
	}

	// Calculate layout and create parameters
	var params []hir.Param
	var offsets []int
	currentOffset := 0

	for i, field := range sd.Fields {
		var fieldType types.T
		if st != nil && i < len(st.Fields) {
			fieldType = st.Fields[i].Type
		}

		// Calculate size and offset
		size := getSize(fieldType)
		offsets = append(offsets, currentOffset)
		currentOffset += size

		// Determine LLVM type for parameter
		llvmType := lowerType(fieldType)

		params = append(params, hir.Param{
			Name: field.Name.Name,
			Type: llvmType,
		})
	}
	b.Func().Params = params
	b.Func().RetType = "ptr"

	structSize := currentOffset
	if structSize == 0 {
		structSize = 1 // Minimum allocation
	}

	// Create entry block
	entry := hir.NewBlock("entry")

	// Allocate space for struct using malloc
	structPtr := hir.Temp{Name: "%struct_ptr"}
	entry.Stmts = append(entry.Stmts, &hir.Call{
		Dst:  structPtr,
		Fn:   "malloc",
		Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", structSize)}},
		Type: "ptr",
	})

	// Store each field
	for i, field := range sd.Fields {
		paramVar := hir.Var{Name: field.Name.Name}
		offset := offsets[i]

		// GEP to field offset
		fieldPtr := hir.Temp{Name: fmt.Sprintf("%%field_%s_ptr", field.Name.Name)}
		entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
			Type:    "i8",
			Base:    structPtr,
			Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
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

	b.Func().Blocks = []*hir.Block{entry}
	return b.Func()
}

// getSize returns the size in bytes for a given type
func getSize(t types.T) int {
	if t == nil {
		return 8 // default to pointer size
	}

	// Handle struct types (pointers)
	if _, ok := t.(*types.Struct); ok {
		return 8
	}

	name := t.String()
	switch name {
	case "int", "i32", "u32":
		return 4
	case "i64", "u64", "isize", "usize":
		return 8
	case "bool":
		return 1
	case "float", "f64":
		return 8
	case "f32":
		return 4
	case "str":
		return 8 // ptr
	default:
		return 8 // pointers, etc.
	}
}
