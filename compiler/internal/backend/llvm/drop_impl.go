package llvm

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// emitDrop generates cleanup code for a value based on its type
func (m *Module) emitDrop(x *hir.Drop) {
	// If no type information, can't do anything
	if x.Type == nil {
		return
	}

	varName, ok := x.Val.(hir.Var)
	if !ok {
		return // Can only drop variables
	}

	// Check if variable was moved
	if m.currentMoves != nil && m.currentMoves[varName.Name] {
		return // Skip drop for moved variable
	}

	t, ok := x.Type.(types.T)
	if !ok {
		return
	}

	if !isHeapType(t) {
		return
	}

	// Get the actual value to free (handles SSA lookup automatically)
	_, ptrValue := m.operand(varName)

	// If the returned value starts with %%, it's a variable reference
	// We need to load it (mutable variable case)
	var ptrToFree string
	if len(ptrValue) > 0 && ptrValue[0] == '%' && m.ssa[varName.Name] == nil {
		// Mutable variable case: need to load from alloca
		loadTemp := fmt.Sprintf("%%drop_load_%d", m.tempID)
		m.tempID++
		wprintf(&m.funcs, "  %s = load ptr, ptr %s\n", loadTemp, ptrValue)
		ptrToFree = loadTemp
	} else {
		// SSA value or immediate
		ptrToFree = ptrValue
	}

	m.emitDropForType(ptrToFree, t)
}

// emitDropForType generates cleanup code for a value of a specific type
func (m *Module) emitDropForType(val string, t types.T) {
	// Check for null before dropping
	// if (val == null) return;
	cond := fmt.Sprintf("%%drop_cond_%d", m.tempID)
	m.tempID++
	wprintf(&m.funcs, "  %s = icmp eq ptr %s, null\n", cond, val)

	doneLabel := fmt.Sprintf("drop_done_%d", m.mergeID)
	doLabel := fmt.Sprintf("drop_do_%d", m.mergeID)
	m.mergeID++

	wprintf(&m.funcs, "  br i1 %s, label %%%s, label %%%s\n\n", cond, doneLabel, doLabel)
	wprintf(&m.funcs, "%s:\n", doLabel)

	switch t := t.(type) {
	case *types.Struct:
		m.emitStructDrop(val, t)
	case *types.Enum:
		m.emitEnumDrop(val, t)
	case *types.List:
		// Call list_free for proper cleanup
		wprintf(&m.funcs, "  call void @list_free(ptr %s)\n", val)
		m.ensureDecl("declare void @list_free(ptr)")
	case *types.Set:
		// Call set_free for proper cleanup
		wprintf(&m.funcs, "  call void @set_free(ptr %s)\n", val)
		m.ensureDecl("declare void @set_free(ptr)")
	case *types.Dict:
		// Call dict_free for proper cleanup
		wprintf(&m.funcs, "  call void @dict_free(ptr %s)\n", val)
		m.ensureDecl("declare void @dict_free(ptr)")
	default:
		// Simple free for other heap types (str, etc)
		wprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
		m.ensureDecl("declare void @free(ptr)")
	}

	wprintf(&m.funcs, "  br label %%%s\n\n", doneLabel)
	wprintf(&m.funcs, "%s:\n", doneLabel)
}

// emitStructDrop generates cleanup code for a struct value
func (m *Module) emitStructDrop(val string, st *types.Struct) {
	// Recursively drop heap-allocated fields using offset-based GEP
	for i, field := range st.Fields {
		if !isHeapType(field.Type) {
			continue
		}

		// Calculate byte offset for this field
		offset := calculateFieldOffset(st, i)

		// GEP to field at byte offset (works with opaque pointers)
		fieldPtr := fmt.Sprintf("%%field_ptr_%d_%d", m.tempID, i)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = getelementptr i8, ptr %s, i32 %d\n",
			fieldPtr, val, offset)

		// Load field value (ptr type)
		fieldVal := fmt.Sprintf("%%field_val_%d_%d", m.tempID, i)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = load ptr, ptr %s\n", fieldVal, fieldPtr)

		// Recursively drop field
		m.emitDropForType(fieldVal, field.Type)
	}

	// Free the struct itself
	fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
	m.ensureDecl("declare void @free(ptr)")
	m.ensureDecl("declare i32 @printf(ptr, ...)")
}

// emitEnumDrop generates cleanup code for an enum value
// Enum layout: tag (i32 at offset 0), payload (ptr at offset 8)
func (m *Module) emitEnumDrop(val string, et *types.Enum) {
	// Check if any variant has heap-allocated fields
	hasHeapPayload := false
	for _, v := range et.Variants {
		for _, f := range v.Fields {
			if isHeapType(f.Type) {
				hasHeapPayload = true
				break
			}
		}
		if hasHeapPayload {
			break
		}
	}

	// If no variant has heap-allocated payload FIELDS, free the payload box
	// (constructors malloc an 8-byte box per payload; unit variants store a
	// null slot) and then the enum itself.
	if !hasHeapPayload {
		m.emitEnumPayloadBoxFree(val)
		fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
		m.ensureDecl("declare void @free(ptr)")
		return
	}

	// Load the tag to determine which variant to drop
	tagPtr := fmt.Sprintf("%%enum_tag_ptr_%d", m.tempID)
	m.tempID++
	fmt.Fprintf(&m.funcs, "  %s = getelementptr i8, ptr %s, i32 0\n", tagPtr, val)

	tagVal := fmt.Sprintf("%%enum_tag_%d", m.tempID)
	m.tempID++
	fmt.Fprintf(&m.funcs, "  %s = load i32, ptr %s\n", tagVal, tagPtr)

	// Generate switch on tag for variants with heap payloads
	doneLabel := fmt.Sprintf("enum_drop_done_%d", m.mergeID)
	defaultLabel := fmt.Sprintf("enum_drop_default_%d", m.mergeID)
	m.mergeID++

	// Build switch cases for variants with heap fields
	var cases []string
	variantLabels := make(map[int]string)

	for _, v := range et.Variants {
		hasHeap := false
		for _, f := range v.Fields {
			if isHeapType(f.Type) {
				hasHeap = true
				break
			}
		}
		if hasHeap {
			label := fmt.Sprintf("enum_drop_v%d_%d", v.Tag, m.mergeID)
			variantLabels[v.Tag] = label
			cases = append(cases, fmt.Sprintf("i32 %d, label %%%s", v.Tag, label))
		}
	}

	// Emit switch instruction
	fmt.Fprintf(&m.funcs, "  switch i32 %s, label %%%s [\n", tagVal, defaultLabel)
	for _, c := range cases {
		fmt.Fprintf(&m.funcs, "    %s\n", c)
	}
	fmt.Fprintf(&m.funcs, "  ]\n\n")

	// Emit code for each variant with heap payloads
	for _, v := range et.Variants {
		label, ok := variantLabels[v.Tag]
		if !ok {
			continue
		}

		fmt.Fprintf(&m.funcs, "%s:\n", label)

		// Load payload pointer from offset 8
		payloadPtr := fmt.Sprintf("%%enum_payload_ptr_%d_%d", v.Tag, m.tempID)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = getelementptr i8, ptr %s, i32 8\n", payloadPtr, val)

		payloadVal := fmt.Sprintf("%%enum_payload_%d_%d", v.Tag, m.tempID)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = load ptr, ptr %s\n", payloadVal, payloadPtr)

		// Null check for payload
		payloadCond := fmt.Sprintf("%%enum_payload_null_%d", m.tempID)
		m.tempID++
		fmt.Fprintf(&m.funcs, "  %s = icmp eq ptr %s, null\n", payloadCond, payloadVal)

		dropPayloadLabel := fmt.Sprintf("enum_drop_payload_%d_%d", v.Tag, m.mergeID)
		skipPayloadLabel := fmt.Sprintf("enum_skip_payload_%d_%d", v.Tag, m.mergeID)
		m.mergeID++

		fmt.Fprintf(&m.funcs, "  br i1 %s, label %%%s, label %%%s\n\n", payloadCond, skipPayloadLabel, dropPayloadLabel)

		// Drop payload fields
		fmt.Fprintf(&m.funcs, "%s:\n", dropPayloadLabel)

		// For each heap field in the variant, recursively drop
		offset := 0
		for i, f := range v.Fields {
			if isHeapType(f.Type) {
				fieldPtr := fmt.Sprintf("%%v%d_field_ptr_%d_%d", v.Tag, i, m.tempID)
				m.tempID++
				fmt.Fprintf(&m.funcs, "  %s = getelementptr i8, ptr %s, i32 %d\n", fieldPtr, payloadVal, offset)

				fieldVal := fmt.Sprintf("%%v%d_field_val_%d_%d", v.Tag, i, m.tempID)
				m.tempID++
				fmt.Fprintf(&m.funcs, "  %s = load ptr, ptr %s\n", fieldVal, fieldPtr)

				m.emitDropForType(fieldVal, f.Type)
			}
			// All fields are 8 bytes (pointer-sized or padded)
			offset += 8
		}

		// Free the payload struct itself
		fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", payloadVal)
		m.ensureDecl("declare void @free(ptr)")

		fmt.Fprintf(&m.funcs, "  br label %%%s\n\n", skipPayloadLabel)

		fmt.Fprintf(&m.funcs, "%s:\n", skipPayloadLabel)
		fmt.Fprintf(&m.funcs, "  br label %%%s\n\n", doneLabel)
	}

	// Default case: variant without heap fields — still free its payload
	// box (null for unit variants; the helper null-checks)
	fmt.Fprintf(&m.funcs, "%s:\n", defaultLabel)
	m.emitEnumPayloadBoxFree(val)
	fmt.Fprintf(&m.funcs, "  br label %%%s\n\n", doneLabel)

	// Done: free the enum wrapper
	fmt.Fprintf(&m.funcs, "%s:\n", doneLabel)
	fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", val)
	m.ensureDecl("declare void @free(ptr)")
}

// emitEnumPayloadBoxFree loads the payload box pointer from enum offset 8,
// null-checks it, and frees it. Constructors always malloc payload boxes
// (or store ptr null for unit variants), so the box itself is heap-owned
// regardless of the payload's field types.
func (m *Module) emitEnumPayloadBoxFree(val string) {
	slot := fmt.Sprintf("%%enum_pbox_slot_%d", m.tempID)
	m.tempID++
	fmt.Fprintf(&m.funcs, "  %s = getelementptr i8, ptr %s, i32 8\n", slot, val)

	box := fmt.Sprintf("%%enum_pbox_%d", m.tempID)
	m.tempID++
	fmt.Fprintf(&m.funcs, "  %s = load ptr, ptr %s\n", box, slot)

	cond := fmt.Sprintf("%%enum_pbox_null_%d", m.tempID)
	m.tempID++
	fmt.Fprintf(&m.funcs, "  %s = icmp eq ptr %s, null\n", cond, box)

	skipLabel := fmt.Sprintf("enum_pbox_skip_%d", m.mergeID)
	freeLabel := fmt.Sprintf("enum_pbox_free_%d", m.mergeID)
	m.mergeID++

	fmt.Fprintf(&m.funcs, "  br i1 %s, label %%%s, label %%%s\n\n", cond, skipLabel, freeLabel)
	fmt.Fprintf(&m.funcs, "%s:\n", freeLabel)
	fmt.Fprintf(&m.funcs, "  call void @free(ptr %s)\n", box)
	m.ensureDecl("declare void @free(ptr)")
	fmt.Fprintf(&m.funcs, "  br label %%%s\n\n", skipLabel)
	fmt.Fprintf(&m.funcs, "%s:\n", skipLabel)
}

func isHeapType(t types.T) bool {
	switch t.(type) {
	case *types.Struct, *types.Enum, *types.List, *types.Dict, *types.Set:
		return true
	}
	return false
}
