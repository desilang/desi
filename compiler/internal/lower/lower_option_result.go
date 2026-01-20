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

	case "unwrap_or":
		// If tag == 0 (Some), return payload. If tag == 1 (None), return default.
		// args[0] is the default value

		if len(args) < 1 {
			return hir.ConstInt{Text: "0"}
		}

		// Lower the default argument first
		defaultVal := ls.lowerExpr(args[0])

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

		// Check: tag == 0 means Some
		isSome := ls.b.FreshTemp("is_some")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isSome,
			Type: "i1",
		})

		// Get element type
		recvType := ls.info.Types[x.X]
		var elemType types.T
		if g, ok := recvType.(*types.Generic); ok {
			if len(g.Args) > 0 {
				elemType = g.Args[0]
			}
		} else if e, ok := recvType.(*types.Enum); ok {
			if len(e.Variants) > 0 && len(e.Variants[0].Fields) > 0 {
				elemType = e.Variants[0].Fields[0].Type
			}
		}

		if elemType == nil {
			return defaultVal
		}

		valType := lowerType(elemType)

		// Allocate result slot on stack
		resultSlot := ls.b.FreshTemp("unwrap_or_slot")
		ls.b.Emit(&hir.Alloca{Type: valType, Dst: resultSlot})

		// Store default first (will be overwritten if Some)
		ls.b.Emit(&hir.Store{Val: defaultVal, Dst: resultSlot})

		// Create conditional block for Some case
		curBlock := ls.b.Block()
		someBlock := ls.b.NewBlock("unwrap_or_some")
		ls.b.SetBlock(someBlock)

		// In Some block: load payload and store to result
		payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     payloadPtrSlot,
		})
		payloadPtr := ls.b.FreshTemp("payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadPtrSlot, Dst: payloadPtr})

		val := ls.b.FreshTemp("unwrap_or_val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})

		ls.b.Emit(&hir.Store{Val: val, Dst: resultSlot})

		// Switch back and emit conditional
		ls.b.SetBlock(curBlock)
		ls.b.Emit(&hir.If{Cond: isSome, Then: someBlock, Else: nil})

		// Load result from slot
		result := ls.b.FreshTemp("unwrap_or_result")
		ls.b.Emit(&hir.Load{Type: valType, Src: resultSlot, Dst: result})
		return result

	case "expect":
		// If tag == 0 (Some), return payload. If tag == 1 (None), panic with custom msg.
		// args[0] is the message

		if len(args) < 1 {
			return hir.ConstInt{Text: "0"}
		}

		// Lower the message argument
		msgVal := ls.lowerExpr(args[0])

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

		// Create panic block
		curBlock := ls.b.Block()
		panicBlock := ls.b.NewBlock("expect_panic")
		ls.b.SetBlock(panicBlock)
		ls.b.Emit(&hir.Call{Fn: "__panic_expect", Args: []hir.Value{msgVal}})

		// Switch back and emit conditional
		ls.b.SetBlock(curBlock)
		ls.b.Emit(&hir.If{Cond: isNone, Then: panicBlock, Else: nil})

		// Get payload
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
			if len(e.Variants) > 0 && len(e.Variants[0].Fields) > 0 {
				elemType = e.Variants[0].Fields[0].Type
			}
		}

		if elemType == nil {
			return hir.ConstInt{Text: "0"}
		}

		valType := lowerType(elemType)
		val := ls.b.FreshTemp("expect_val")
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

	case "unwrap_or":
		// If tag == 0 (Ok), return payload. If tag == 1 (Err), return default.
		// args[0] is the default value

		if len(args) < 1 {
			return hir.ConstInt{Text: "0"}
		}

		// Lower the default argument first
		defaultVal := ls.lowerExpr(args[0])

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

		// Check: tag == 0 means Ok
		isOk := ls.b.FreshTemp("is_ok")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isOk,
			Type: "i1",
		})

		// Get element type (T from Result<T, E>)
		recvType := ls.info.Types[x.X]
		var elemType types.T
		if g, ok := recvType.(*types.Generic); ok {
			if len(g.Args) > 0 {
				elemType = g.Args[0] // T
			}
		} else if e, ok := recvType.(*types.Enum); ok {
			if len(e.Variants) > 0 && len(e.Variants[0].Fields) > 0 {
				elemType = e.Variants[0].Fields[0].Type
			}
		}

		if elemType == nil {
			return defaultVal
		}

		valType := lowerType(elemType)

		// Allocate result slot on stack
		resultSlot := ls.b.FreshTemp("unwrap_or_slot")
		ls.b.Emit(&hir.Alloca{Type: valType, Dst: resultSlot})

		// Store default first (will be overwritten if Ok)
		ls.b.Emit(&hir.Store{Val: defaultVal, Dst: resultSlot})

		// Create conditional block for Ok case
		curBlock := ls.b.Block()
		okBlock := ls.b.NewBlock("result_unwrap_or_ok")
		ls.b.SetBlock(okBlock)

		// In Ok block: load payload and store to result
		payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     payloadPtrSlot,
		})
		payloadPtr := ls.b.FreshTemp("payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadPtrSlot, Dst: payloadPtr})

		val := ls.b.FreshTemp("unwrap_or_val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})

		ls.b.Emit(&hir.Store{Val: val, Dst: resultSlot})

		// Switch back and emit conditional
		ls.b.SetBlock(curBlock)
		ls.b.Emit(&hir.If{Cond: isOk, Then: okBlock, Else: nil})

		// Load result from slot
		result := ls.b.FreshTemp("unwrap_or_result")
		ls.b.Emit(&hir.Load{Type: valType, Src: resultSlot, Dst: result})
		return result

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

	case "expect":
		// If tag == 0 (Ok), return payload. If tag == 1 (Err), panic with custom msg.
		if len(args) < 1 {
			return hir.ConstInt{Text: "0"}
		}

		msgVal := ls.lowerExpr(args[0])

		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		isErr := ls.b.FreshTemp("is_err")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "!=",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isErr,
			Type: "i1",
		})

		curBlock := ls.b.Block()
		panicBlock := ls.b.NewBlock("expect_panic")
		ls.b.SetBlock(panicBlock)
		ls.b.Emit(&hir.Call{Fn: "__panic_expect", Args: []hir.Value{msgVal}})

		ls.b.SetBlock(curBlock)
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
				elemType = g.Args[0]
			}
		} else if e, ok := recvType.(*types.Enum); ok {
			if len(e.Variants) > 0 && len(e.Variants[0].Fields) > 0 {
				elemType = e.Variants[0].Fields[0].Type
			}
		}

		if elemType == nil {
			return hir.ConstInt{Text: "0"}
		}

		valType := lowerType(elemType)
		val := ls.b.FreshTemp("expect_val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})
		return val

	case "expect_err":
		// If tag == 1 (Err), return payload. If tag == 0 (Ok), panic with custom msg.
		if len(args) < 1 {
			return hir.ConstInt{Text: "0"}
		}

		msgVal := ls.lowerExpr(args[0])

		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		isOk := ls.b.FreshTemp("is_ok")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isOk,
			Type: "i1",
		})

		curBlock := ls.b.Block()
		panicBlock := ls.b.NewBlock("expect_err_panic")
		ls.b.SetBlock(panicBlock)
		ls.b.Emit(&hir.Call{Fn: "__panic_expect", Args: []hir.Value{msgVal}})

		ls.b.SetBlock(curBlock)
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
				errType = g.Args[1]
			}
		} else if e, ok := recvType.(*types.Enum); ok {
			if len(e.Variants) > 1 && len(e.Variants[1].Fields) > 0 {
				errType = e.Variants[1].Fields[0].Type
			}
		}

		if errType == nil {
			return hir.ConstInt{Text: "0"}
		}

		valType := lowerType(errType)
		val := ls.b.FreshTemp("expect_err_val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})
		return val

	case "ok":
		// Result.ok() -> Option<T>
		// If Ok(v), return Some(v). If Err(_), return Nothing.

		// Get tag from Result
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		// Allocate Option struct
		optionSlot := ls.b.FreshTemp("option_slot")
		ls.b.Emit(&hir.Alloca{Type: "{i32, ptr}", Count: 1, Dst: optionSlot})

		// Pre-store Nothing (tag = 1)
		tagPtrOption := ls.b.FreshTemp("tag_ptr_option")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    optionSlot,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtrOption,
		})
		ls.b.Emit(&hir.Store{Dst: tagPtrOption, Val: hir.ConstInt{Text: "1", Type: "i32"}})

		// Check if Ok (tag == 0)
		isOk := ls.b.FreshTemp("is_ok")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isOk,
			Type: "i1",
		})

		// Create conditional block for Ok case
		curBlock := ls.b.Block()
		okBlock := ls.b.NewBlock("ok_some")
		ls.b.SetBlock(okBlock)

		// In Ok block: overwrite with Some (tag=0) + copy payload
		ls.b.Emit(&hir.Store{Dst: tagPtrOption, Val: hir.ConstInt{Text: "0", Type: "i32"}})
		srcPayloadPtrSlot := ls.b.FreshTemp("src_payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     srcPayloadPtrSlot,
		})
		srcPayloadPtr := ls.b.FreshTemp("src_payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: srcPayloadPtrSlot, Dst: srcPayloadPtr})
		dstPayloadPtrSlot := ls.b.FreshTemp("dst_payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    optionSlot,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     dstPayloadPtrSlot,
		})
		ls.b.Emit(&hir.Store{Dst: dstPayloadPtrSlot, Val: srcPayloadPtr})

		// Switch back and emit conditional (Else: nil = fallthrough)
		ls.b.SetBlock(curBlock)
		ls.b.Emit(&hir.If{Cond: isOk, Then: okBlock, Else: nil})

		return optionSlot

	case "err":
		// Result.err() -> Option<E>
		// If Err(e), return Some(e). If Ok(_), return Nothing.

		// Get tag from Result
		tagPtr := ls.b.FreshTemp("tag_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtr,
		})
		tag := ls.b.FreshTemp("tag")
		ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

		// Allocate Option struct
		optionSlot := ls.b.FreshTemp("option_slot")
		ls.b.Emit(&hir.Alloca{Type: "{i32, ptr}", Count: 1, Dst: optionSlot})

		// Pre-store Nothing (tag = 1)
		tagPtrOption := ls.b.FreshTemp("tag_ptr_option")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    optionSlot,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}},
			Dst:     tagPtrOption,
		})
		ls.b.Emit(&hir.Store{Dst: tagPtrOption, Val: hir.ConstInt{Text: "1", Type: "i32"}})

		// Check if Err (tag != 0)
		isErr := ls.b.FreshTemp("is_err")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "!=",
			LHS:  tag,
			RHS:  hir.ConstInt{Text: "0", Type: "i32"},
			Dst:  isErr,
			Type: "i1",
		})

		// Create conditional block for Err case
		curBlock := ls.b.Block()
		errBlock := ls.b.NewBlock("err_some")
		ls.b.SetBlock(errBlock)

		// In Err block: overwrite with Some (tag=0) + copy payload
		ls.b.Emit(&hir.Store{Dst: tagPtrOption, Val: hir.ConstInt{Text: "0", Type: "i32"}})
		srcPayloadPtrSlot := ls.b.FreshTemp("src_payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    receiver,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     srcPayloadPtrSlot,
		})
		srcPayloadPtr := ls.b.FreshTemp("src_payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: srcPayloadPtrSlot, Dst: srcPayloadPtr})
		dstPayloadPtrSlot := ls.b.FreshTemp("dst_payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    optionSlot,
			Indices: []hir.Value{hir.ConstInt{Text: "8"}},
			Dst:     dstPayloadPtrSlot,
		})
		ls.b.Emit(&hir.Store{Dst: dstPayloadPtrSlot, Val: srcPayloadPtr})

		// Switch back and emit conditional (Else: nil = fallthrough)
		ls.b.SetBlock(curBlock)
		ls.b.Emit(&hir.If{Cond: isErr, Then: errBlock, Else: nil})

		return optionSlot
	}

	return hir.ConstInt{Text: "0"}
}
