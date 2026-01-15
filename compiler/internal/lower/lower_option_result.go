package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerOptionMethod(x *ast.FieldExpr, args []ast.Expr, t *types.Enum) hir.Value {
	method := x.Name.Name
	receiver := ls.lowerExpr(x.X)

	switch method {
	case "is_some":
		// Check tag == 0
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		res := ls.b.FreshTemp("is_some")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0"},
			Dst:  res,
			Type: "i1",
		})
		return res

	case "is_none", "is_nothing":
		// Check tag == 1
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		res := ls.b.FreshTemp("is_none")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "1"},
			Dst:  res,
			Type: "i1",
		})
		return res

	case "unwrap":
		// If tag == 0, return payload. Else panic.
		// For now, just assume tag == 0 and load payload.
		// TODO: Add runtime check and panic

		// Get payload ptr at offset 8 (aligned after i32 tag + padding)
		payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     payloadPtrSlot,
		})
		payloadPtr := ls.b.FreshTemp("payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadPtrSlot, Dst: payloadPtr})

		// Load value from payload ptr
		// We need the element type to load it
		// t is Option<T>, so we need T.
		// But t passed here is *types.Enum, which might be the generic definition or instantiated?
		// In lowerCall, we unwrap Generic to get Enum.
		// We need the instantiated type to know T.

		// If we don't have the instantiated type here, we can't know the return type size to load.
		// But wait, lowerCall passes *types.Enum.
		// If it was a Generic, we need the Generic info.

		// Let's assume for now we return the pointer to the value (borrowed) or load it?
		// If T is primitive (int), we load it.
		// If T is ptr (str, class), we load the ptr.

		// We need the return type of unwrap() which is T.
		// We can get it from ls.info.Types[x.X] (the receiver type).

		recvType := ls.info.Types[x.X]
		var elemType types.T
		if g, ok := recvType.(*types.Generic); ok {
			if len(g.Args) > 0 {
				elemType = g.Args[0]
			}
		} else if e, ok := recvType.(*types.Enum); ok {
			// If it's not generic (e.g. specialized enum), check fields
			if len(e.Variants) > 0 && len(e.Variants[0].Fields) > 0 {
				elemType = e.Variants[0].Fields[0].Type
			}
		}

		if elemType == nil {
			// Should not happen for Option
			return hir.ConstInt{Text: "0"}
		}

		valType := lowerType(elemType)
		val := ls.b.FreshTemp("val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})
		return val
	}

	return hir.ConstInt{Text: "0"}
}

func (ls *lowerState) lowerResultMethod(x *ast.FieldExpr, args []ast.Expr, t *types.Enum) hir.Value {
	method := x.Name.Name
	receiver := ls.lowerExpr(x.X)

	switch method {
	case "is_ok":
		// Check tag == 0
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		res := ls.b.FreshTemp("is_ok")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0"},
			Dst:  res,
			Type: "i1",
		})
		return res

	case "is_err":
		// Check tag == 1
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		res := ls.b.FreshTemp("is_err")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "1"},
			Dst:  res,
			Type: "i1",
		})
		return res

	case "unwrap":
		// If tag == 0, return payload. Else panic.
		// TODO: Panic on error

		payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     payloadPtrSlot,
		})
		payloadPtr := ls.b.FreshTemp("payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadPtrSlot, Dst: payloadPtr})

		recvType := ls.info.Types[x.X]
		var elemType types.T
		if g, ok := recvType.(*types.Generic); ok {
			if len(g.Args) > 0 {
				elemType = g.Args[0] // T
			}
		} else if e, ok := recvType.(*types.Enum); ok {
			// Fall back to Enum variant fields (Ok variant = index 0)
			if len(e.Variants) > 0 && len(e.Variants[0].Fields) > 0 {
				elemType = e.Variants[0].Fields[0].Type
			}
		}

		if elemType == nil {
			// Last resort: use the passed in enum type
			if t != nil && len(t.Variants) > 0 && len(t.Variants[0].Fields) > 0 {
				elemType = t.Variants[0].Fields[0].Type
			}
		}

		if elemType == nil {
			return hir.ConstInt{Text: "0"}
		}

		valType := lowerType(elemType)
		val := ls.b.FreshTemp("val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})
		return val

	case "unwrap_err":
		// If tag == 1, return payload. Else panic.
		// TODO: Panic on ok

		payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     payloadPtrSlot,
		})
		payloadPtr := ls.b.FreshTemp("payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadPtrSlot, Dst: payloadPtr})

		recvType := ls.info.Types[x.X]
		var errType types.T
		if g, ok := recvType.(*types.Generic); ok {
			if len(g.Args) > 1 {
				errType = g.Args[1] // E
			}
		} else if e, ok := recvType.(*types.Enum); ok {
			// Fall back to Enum variant fields (Err variant = index 1)
			if len(e.Variants) > 1 && len(e.Variants[1].Fields) > 0 {
				errType = e.Variants[1].Fields[0].Type
			}
		}

		if errType == nil {
			// Last resort: use the passed in enum type
			if t != nil && len(t.Variants) > 1 && len(t.Variants[1].Fields) > 0 {
				errType = t.Variants[1].Fields[0].Type
			}
		}

		if errType == nil {
			return hir.ConstInt{Text: "0"}
		}

		valType := lowerType(errType)
		val := ls.b.FreshTemp("err_val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})
		return val
	}

	return hir.ConstInt{Text: "0"}
}
