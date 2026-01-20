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
		// If tag == 0 (Some), return payload. If tag == 1 (None), panic.

		// Get tag
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		// Check: tag != 0 means None - should panic
		isNone := ls.b.FreshTemp("is_none")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "!=",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isNone,
			Type: "i1",
		})

		// Create panic block using builder (properly registers with function)
		curBlock := ls.b.Block()
		panicBlock := ls.b.NewBlock("unwrap_panic")
		ls.b.SetBlock(panicBlock)
		ls.b.Emit(&hir.Call{Fn: "__panic_unwrap_none", Args: nil})
		// Note: after exit(1), code is unreachable, but we need a terminator
		// The backend handles this - the If statement structure provides proper control flow

		// Switch back to current block and emit the If
		ls.b.SetBlock(curBlock)

		// Emit conditional: if (isNone) { panic }
		ls.b.Emit(&hir.If{Cond: isNone, Then: panicBlock, Else: nil})

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
		// If tag == 0 (Ok), return payload. If tag == 1 (Err), panic.

		// Get tag
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		// Check: tag != 0 means Err - should panic
		isErr := ls.b.FreshTemp("is_err_check")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "!=",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isErr,
			Type: "i1",
		})

		// Create panic block using builder (properly registers with function)
		curBlock := ls.b.Block()
		panicBlock := ls.b.NewBlock("result_unwrap_panic")
		ls.b.SetBlock(panicBlock)
		ls.b.Emit(&hir.Call{Fn: "__panic_unwrap_err", Args: nil})

		// Switch back to current block and emit the If
		ls.b.SetBlock(curBlock)

		// Emit conditional: if (isErr) { panic }
		ls.b.Emit(&hir.If{Cond: isErr, Then: panicBlock, Else: nil})

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
		// If tag == 1 (Err), return payload. If tag == 0 (Ok), panic.

		// Get tag
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		// Check: tag != 1 means Ok - should panic
		isOk := ls.b.FreshTemp("is_ok_check")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "!=",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "1", Type: "i32"},
			Dst:  isOk,
			Type: "i1",
		})

		// Create panic block using builder (properly registers with function)
		curBlock := ls.b.Block()
		panicBlock := ls.b.NewBlock("result_unwrap_err_panic")
		ls.b.SetBlock(panicBlock)
		ls.b.Emit(&hir.Call{Fn: "__panic_unwrap_ok", Args: nil})

		// Switch back to current block and emit the If
		ls.b.SetBlock(curBlock)

		// Emit conditional: if (isOk) { panic }
		ls.b.Emit(&hir.If{Cond: isOk, Then: panicBlock, Else: nil})

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
