package llvm

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/types"
)

// EmitTypeDefs emits LLVM type definitions for structs and enums
// This must be called before emitting functions that reference these types
func (m *Module) EmitTypeDefs(structDecls []*ast.StructDecl, enumDecls []*ast.EnumDecl, info *check.Info) {
	// Convert AST structs to types.Struct using info.Types
	var structs []*types.Struct
	for _, sd := range structDecls {
		// Look up the type info for this struct
		// We'll need to access it from the scope via the resolver
		// For now, reconstruct from AST
		var fields []types.Field
		for _, f := range sd.Fields {
			// Get field type from check info if available
			var fieldType types.T
			if info != nil && f.Type != nil {
				// Try to resolve the type name
				fieldType = resolveTypeFromName(f.Type.Name)
			}
			if fieldType == nil {
				fieldType = types.Int // default fallback
			}
			fields = append(fields, types.Field{
				Name: f.Name.Name,
				Type: fieldType,
			})
		}
		structs = append(structs, &types.Struct{
			Name:   sd.Name.Name,
			Fields: fields,
		})
	}

	// Convert AST enums to types.Enum
	var enums []*types.Enum
	for _, ed := range enumDecls {
		var variants []types.Variant
		for i, v := range ed.Variants {
			var fields []types.Field
			// Enum variants have an optional single payload type
			if v.Type != nil {
				var fieldType types.T
				if info != nil {
					fieldType = resolveTypeFromName(v.Type.Name)
				}
				if fieldType == nil {
					fieldType = types.Int // default fallback
				}
				// Single payload field
				fields = append(fields, types.Field{
					Name: "payload",
					Type: fieldType,
				})
			}
			variants = append(variants, types.Variant{
				Name:   v.Name.Name,
				Fields: fields,
				Tag:    i,
			})
		}
		enums = append(enums, &types.Enum{
			Name:     ed.Name.Name,
			Variants: variants,
		})
	}

	// Actually emit the type definitions
	emitStructTypeDefs(m, structs, enums)
}

// resolveTypeFromName is a simple helper to convert type names to types.T
func resolveTypeFromName(name string) types.T {
	if t, ok := types.FromName(name); ok {
		return t
	}
	// For custom types, we'd need to look them up in the scope
	// For now, return ptr for unknown types
	return nil
}

// emitStructTypeDefs emits the actual LLVM type definitions
func emitStructTypeDefs(m *Module, structs []*types.Struct, enums []*types.Enum) {
	// Emit struct types
	for _, st := range structs {
		var fieldTypes []string
		for _, f := range st.Fields {
			fieldTypes = append(fieldTypes, llvmType(f.Type))
		}
		// %StructName = type { field1_type, field2_type, ... }
		if len(fieldTypes) == 0 {
			wprintf(&m.globals, "%%%s = type {}\n", st.Name)
		} else {
			wprintf(&m.globals, "%%%s = type { %s }\n", st.Name, joinTypes(fieldTypes))
		}
	}

	// Emit enum types
	// Enums have layout: { i32 tag, ptr payload }
	for _, et := range enums {
		wprintf(&m.globals, "%%%s = type { i32, ptr }\n", et.Name)
	}
}
