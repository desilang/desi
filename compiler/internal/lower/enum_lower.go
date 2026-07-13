package lower

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// LowerEnumConstructors generates constructor functions for each enum variant.
// Constructor signature for each variant: EnumName.VariantName(fields...) -> ptr
// Memory layout: struct { tag: i32, payload: ptr }
func LowerEnumConstructors(ed *ast.EnumDecl, info *check.Info) []*hir.Func {
	enumName := ed.Name.Name
	var funcs []*hir.Func

	// Look up enum type to get resolved variant types
	var et *types.Enum
	if t := info.Types[ed]; t != nil {
		et, _ = t.(*types.Enum)
	}

	for i, v := range ed.Variants {
		variantName := v.Name.Name
		funcName := enumName + "." + variantName

		// Get variant type info
		var variant *types.Variant
		if et != nil && i < len(et.Variants) {
			variant = &et.Variants[i]
		}

		// Build parameter list (if variant has payload)
		var params []hir.Param
		if variant != nil && len(variant.Fields) > 0 {
			// For now, we have a single "value" field containing the payload type
			field := variant.Fields[0]
			llvmType := lowerType(field.Type)
			params = append(params, hir.Param{
				Name: "value",
				Type: llvmType,
			})
		}

		// Create function
		b := hir.NewFunc(funcName)
		b.Func().Params = params
		b.Func().RetType = "ptr" // Return pointer to enum

		entry := hir.NewBlock("entry")

		// Allocate enum struct: 16 bytes (4 for i32 tag + 4 padding + 8 for ptr payload)
		// Pointer must be 8-byte aligned, so payload ptr goes at offset 8, not 4
		enumPtr := hir.Temp{Name: "%enum_ptr"}
		entry.Stmts = append(entry.Stmts, &hir.Call{
			Dst:  enumPtr,
			Fn:   "malloc",
			Args: []hir.Value{hir.ConstInt{Text: "16"}},
			Type: "ptr",
		})

		// Store tag at offset 0
		tagPtr := hir.Temp{Name: "%tag_ptr"}
		entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
			Type:    "i8",
			Base:    enumPtr,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		entry.Stmts = append(entry.Stmts, &hir.Store{
			Dst: tagPtr,
			Val: hir.ConstInt{Text: fmt.Sprintf("%d", i)}, // Tag value
		})

		// Handle payload
		if variant != nil && len(variant.Fields) > 0 {
			// Allocate payload and store value
			field := variant.Fields[0]
			payloadSize := getSize(field.Type)

			payloadPtr := hir.Temp{Name: "%payload_ptr"}
			entry.Stmts = append(entry.Stmts, &hir.Call{
				Dst:  payloadPtr,
				Fn:   "malloc",
				Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", payloadSize)}},
				Type: "ptr",
			})

			// Store the value to payload
			entry.Stmts = append(entry.Stmts, &hir.Store{
				Dst: payloadPtr,
				Val: hir.Var{Name: "value"},
			})

			// Store payload pointer at offset 8 (aligned for 8-byte ptr)
			payloadPtrSlot := hir.Temp{Name: "%payload_ptr_slot"}
			entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
				Type:    "i8",
				Base:    enumPtr,
				Indices: []hir.Value{hir.ConstInt{Text: "8"}},
				Dst:     payloadPtrSlot,
			})
			entry.Stmts = append(entry.Stmts, &hir.Store{
				Dst: payloadPtrSlot,
				Val: payloadPtr,
			})
		} else {
			// Unit variant: set payload to null at offset 8.
			// Must store a full 8-byte ptr null — an i32 0 store leaves the
			// slot's upper 4 bytes as malloc garbage, and the enum drop
			// null-checks this slot before freeing the payload box.
			payloadPtrSlot := hir.Temp{Name: "%payload_ptr_slot"}
			entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
				Type:    "i8",
				Base:    enumPtr,
				Indices: []hir.Value{hir.ConstInt{Text: "8"}},
				Dst:     payloadPtrSlot,
			})
			entry.Stmts = append(entry.Stmts, &hir.Store{
				Dst: payloadPtrSlot,
				Val: hir.ConstNull{},
			})
		}

		// Return enum pointer
		entry.Stmts = append(entry.Stmts, &hir.Ret{Val: enumPtr})

		b.Func().Blocks = []*hir.Block{entry}
		funcs = append(funcs, b.Func())
	}

	return funcs
}

// LowerEnumConstructorsFromType generates constructor functions from a types.Enum directly.
// Used for built-in enums like Option<T> and Result<T,E> that don't have AST declarations.
func LowerEnumConstructorsFromType(enumName string, et *types.Enum) []*hir.Func {
	var funcs []*hir.Func

	for i, variant := range et.Variants {
		funcName := enumName + "." + variant.Name

		// Build parameter list (if variant has payload)
		var params []hir.Param
		if len(variant.Fields) > 0 {
			field := variant.Fields[0]
			llvmType := lowerType(field.Type)
			params = append(params, hir.Param{
				Name: "value",
				Type: llvmType,
			})
		}

		// Create function
		b := hir.NewFunc(funcName)
		b.Func().Params = params
		b.Func().RetType = "ptr" // Return pointer to enum

		entry := hir.NewBlock("entry")

		// Allocate enum struct: 16 bytes (4 for i32 tag + 4 padding + 8 for ptr payload)
		// Pointer must be 8-byte aligned, so payload ptr goes at offset 8, not 4
		enumPtr := hir.Temp{Name: "%enum_ptr"}
		entry.Stmts = append(entry.Stmts, &hir.Call{
			Dst:  enumPtr,
			Fn:   "malloc",
			Args: []hir.Value{hir.ConstInt{Text: "16"}},
			Type: "ptr",
		})

		// Store tag at offset 0
		tagPtr := hir.Temp{Name: "%tag_ptr"}
		entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
			Type:    "i8",
			Base:    enumPtr,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		entry.Stmts = append(entry.Stmts, &hir.Store{
			Dst: tagPtr,
			Val: hir.ConstInt{Text: fmt.Sprintf("%d", i)}, // Tag value
		})

		// Handle payload
		if len(variant.Fields) > 0 {
			field := variant.Fields[0]
			payloadSize := getSize(field.Type)

			payloadPtr := hir.Temp{Name: "%payload_ptr"}
			entry.Stmts = append(entry.Stmts, &hir.Call{
				Dst:  payloadPtr,
				Fn:   "malloc",
				Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", payloadSize)}},
				Type: "ptr",
			})

			// Store the value to payload
			entry.Stmts = append(entry.Stmts, &hir.Store{
				Dst: payloadPtr,
				Val: hir.Var{Name: "value"},
			})

			// Store payload pointer at offset 8 (aligned for 8-byte ptr)
			payloadPtrSlot := hir.Temp{Name: "%payload_ptr_slot"}
			entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
				Type:    "i8",
				Base:    enumPtr,
				Indices: []hir.Value{hir.ConstInt{Text: "8"}},
				Dst:     payloadPtrSlot,
			})
			entry.Stmts = append(entry.Stmts, &hir.Store{
				Dst: payloadPtrSlot,
				Val: payloadPtr,
			})
		} else {
			// Unit variant: set payload to null at offset 8 (full ptr-width
			// null — see comment in LowerEnumConstructors above)
			payloadPtrSlot := hir.Temp{Name: "%payload_ptr_slot"}
			entry.Stmts = append(entry.Stmts, &hir.GetElementPtr{
				Type:    "i8",
				Base:    enumPtr,
				Indices: []hir.Value{hir.ConstInt{Text: "8"}},
				Dst:     payloadPtrSlot,
			})
			entry.Stmts = append(entry.Stmts, &hir.Store{
				Dst: payloadPtrSlot,
				Val: hir.ConstNull{},
			})
		}

		// Return enum pointer
		entry.Stmts = append(entry.Stmts, &hir.Ret{Val: enumPtr})

		b.Func().Blocks = []*hir.Block{entry}
		funcs = append(funcs, b.Func())
	}

	return funcs
}
