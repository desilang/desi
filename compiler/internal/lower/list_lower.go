package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerListMethod(fe *ast.FieldExpr, args []ast.Expr, listType *types.List) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name

	switch method {
	case "append":
		// append(elem) - list takes ownership of elem
		elem := ls.lowerExpr(args[0])

		// Mark variable as moved if it's an identifier (ownership transferred to list)
		if id, ok := args[0].(*ast.Ident); ok {
			ls.cur().moved[id.Name] = true
		}
		// Temp results (string concat etc.) also transfer ownership — the
		// list stores the raw pointer, so a scope-end free would dangle.
		ls.consumeTemp(elem)

		// Cast to ptr for generic storage (void*)
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})

		// Determine type tag
		var typeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
		if ls.info != nil {
			typeTag = getTypeTag(ls.info.Types[args[0]])
		}

		ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{receiver, elemPtr, typeTag}})
		return nil

	case "get":
		// get(index) -> T
		index := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("elem")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_get", Args: []hir.Value{receiver, index}})
		return res

	case "set":
		// set(index, elem)
		index := ls.lowerExpr(args[0])
		elem := ls.lowerExpr(args[1])
		ls.consumeTemp(elem) // list stores the raw pointer

		// Cast to ptr for generic storage
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})

		ls.b.Emit(&hir.Call{Fn: "list_set", Args: []hir.Value{receiver, index, elemPtr}})
		return nil

	case "len":
		// len() -> int
		res64 := ls.b.FreshTemp("len64")
		ls.b.Emit(&hir.Call{Dst: res64, Fn: "list_len", Args: []hir.Value{receiver}})

		res := ls.b.FreshTemp("len")
		ls.b.Emit(&hir.Cast{Dst: res, Src: res64, Type: "i32"})
		return res

	case "pop":
		// pop() -> T
		res := ls.b.FreshTemp("popped")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_pop", Args: []hir.Value{receiver, hir.ConstInt{Text: "-1", Type: "i64"}}})
		return res

	case "free":
		// free()
		ls.b.Emit(&hir.Call{Fn: "list_free", Args: []hir.Value{receiver}})
		return nil

	case "insert":
		// insert(index, elem)
		index := ls.lowerExpr(args[0])
		elem := ls.lowerExpr(args[1])
		ls.consumeTemp(elem) // list stores the raw pointer
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})
		ls.b.Emit(&hir.Call{Fn: "list_insert", Args: []hir.Value{receiver, index, elemPtr}})
		return nil

	case "remove":
		// remove(elem)
		elem := ls.lowerExpr(args[0])
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})
		ls.b.Emit(&hir.Call{Fn: "list_remove", Args: []hir.Value{receiver, elemPtr}})
		return nil

	case "reverse":
		// reverse()
		ls.b.Emit(&hir.Call{Fn: "list_reverse", Args: []hir.Value{receiver}})
		return nil

	case "sort":
		// sort(reverse=false) - in-place sort
		var reverseArg hir.Value = hir.ConstInt{Text: "0", Type: "i32"} // default ascending
		if len(args) > 0 {
			// If arg provided, check if it's a bool literal or expression
			reverseArg = ls.lowerExpr(args[0])
		}
		ls.b.Emit(&hir.Call{Fn: "list_sort", Args: []hir.Value{receiver, reverseArg}})
		return nil

	case "clear":
		// clear()
		ls.b.Emit(&hir.Call{Fn: "list_clear", Args: []hir.Value{receiver}})
		return nil

	case "copy":
		// copy() -> list[T]
		res := ls.b.FreshTemp("copy")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_copy", Args: []hir.Value{receiver}})
		return res

	case "extend":
		// extend(other)
		other := ls.lowerExpr(args[0])
		ls.b.Emit(&hir.Call{Fn: "list_extend", Args: []hir.Value{receiver, other}})
		return nil

	case "index":
		// index(elem) -> int
		elem := ls.lowerExpr(args[0])
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})

		// list_index returns i64, cast to i32
		res64 := ls.b.FreshTemp("idx64")
		// list_index(list, item, start=0, end=len) - simplified wrapper needed or pass args
		// The runtime signature is list_index(list, item, start, end)
		// But here we expose index(elem) -> int. We need to pass defaults.
		// Wait, the runtime signature in C is: int64_t list_index(DesiList* list, void* item, int64_t start, int64_t end);
		// We need to pass 0 and len as defaults.

		// Get length for end arg
		len64 := ls.b.FreshTemp("len64")
		ls.b.Emit(&hir.Call{Dst: len64, Fn: "list_len", Args: []hir.Value{receiver}})

		zero := hir.ConstInt{Text: "0"}
		ls.b.Emit(&hir.Call{Dst: res64, Fn: "list_index", Args: []hir.Value{receiver, elemPtr, zero, len64}})

		res := ls.b.FreshTemp("idx")
		ls.b.Emit(&hir.Cast{Dst: res, Src: res64, Type: "i32"})
		return res

	case "count":
		// count(elem) -> int
		elem := ls.lowerExpr(args[0])
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})

		res64 := ls.b.FreshTemp("count64")
		ls.b.Emit(&hir.Call{Dst: res64, Fn: "list_count", Args: []hir.Value{receiver, elemPtr}})

		res := ls.b.FreshTemp("count")
		ls.b.Emit(&hir.Cast{Dst: res, Src: res64, Type: "i32"})
		return res

	case "contains":
		// contains(elem) -> bool
		elem := ls.lowerExpr(args[0])
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})

		res := ls.b.FreshTemp("found")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_contains", Args: []hir.Value{receiver, elemPtr}})
		return res

	case "to_str":
		// to_str() -> str
		res := ls.b.FreshTemp("str")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_to_str", Args: []hir.Value{receiver}, Type: "ptr"})
		return res

	case "iter":
		// iter() -> ListIter (stack-allocated iterator struct)
		// ListIter in C = {DesiList* list, int64_t index} = {ptr, i64}
		// For LLVM, we allocate on stack and pass address
		iterPtr := ls.b.FreshTemp("iter_ptr")
		ls.b.Emit(&hir.Alloca{Dst: iterPtr, Type: "{ptr, i64}", Count: 1})
		ls.b.Emit(&hir.Call{Fn: "list_iter_init", Args: []hir.Value{iterPtr, receiver}})
		return iterPtr

	case "map":
		// map(fn) -> list[U]
		// Create new list, iterate source, apply fn to each element, append result
		if len(args) < 1 {
			return nil
		}

		// Get the function name from the argument using AST pattern matching
		funcExpr := args[0]
		fnName := ""
		if id, ok := funcExpr.(*ast.Ident); ok {
			fnName = id.Name
		} else if fe, ok := funcExpr.(*ast.FieldExpr); ok {
			// Qualified name like mod.func
			fnName = ls.calleeName(funcExpr)
			_ = fe
		} else {
			// Fallback
			fnName = ls.calleeName(funcExpr)
		}

		// Get return type from type info for proper casting
		fnRetType := "ptr"
		if ls.info != nil {
			if fnType, ok := ls.info.Types[args[0]].(*types.Func); ok {
				fnRetType = lowerType(fnType.Ret)
			}
		}

		// Create result list
		resultList := ls.b.FreshTemp("map_result_list")
		ls.b.Emit(&hir.Call{Dst: resultList, Fn: "list_new", Args: nil})

		// Get length of source list
		srcLen := ls.b.FreshTemp("src_len")
		ls.b.Emit(&hir.Call{Dst: srcLen, Fn: "list_len", Args: []hir.Value{receiver}})

		// Create index pointer (on stack so it persists across blocks)
		idxPtr := ls.b.FreshTemp("idx_ptr")
		ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
		ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

		// Create condition block
		condBlk := ls.b.NewBlock("map_cond")
		oldCur := ls.b.Block()

		ls.b.SetBlock(condBlk)
		idxVal := ls.b.FreshTemp("map_idx")
		ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
		condTemp := ls.b.FreshTemp("map_cond_val")
		ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: srcLen, Type: "i1"})
		ls.b.SetBlock(oldCur)

		// Create body block
		bodyBlk := ls.b.NewBlock("map_body")
		ls.b.SetBlock(bodyBlk)

		// Load current index
		idxBody := ls.b.FreshTemp("map_idx_body")
		ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

		// Get element from source list
		elem := ls.b.FreshTemp("map_elem")
		ls.b.Emit(&hir.Call{Dst: elem, Fn: "list_get", Args: []hir.Value{receiver, idxBody}, Type: "ptr"})

		// Call the function with the element
		fnResult := ls.b.FreshTemp("fn_result")
		ls.b.Emit(&hir.Call{Dst: fnResult, Fn: fnName, Args: []hir.Value{elem}, Type: fnRetType})

		// Cast result and append to result list
		fnResultPtr := ls.b.FreshTemp("fn_result_ptr")
		ls.b.Emit(&hir.Cast{Dst: fnResultPtr, Src: fnResult, Type: "ptr"})
		ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{resultList, fnResultPtr, hir.ConstInt{Text: "0", Type: "i32"}}})

		// Increment index
		nextIdx := ls.b.FreshTemp("next_idx")
		ls.b.Emit(&hir.BinaryOp{Dst: nextIdx, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
		ls.b.Emit(&hir.Store{Dst: idxPtr, Val: nextIdx})

		// Emit while loop from original block
		ls.b.SetBlock(oldCur)
		ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})

		return resultList

	case "filter":
		// filter(fn) -> list[T]
		// Create new list, iterate source, if fn(elem) is true, append elem
		if len(args) < 1 {
			return nil
		}

		// Get the function name from the argument using AST pattern matching
		funcExpr := args[0]
		fnName := ""
		if id, ok := funcExpr.(*ast.Ident); ok {
			fnName = id.Name
		} else if fe, ok := funcExpr.(*ast.FieldExpr); ok {
			// Qualified name like mod.func
			fnName = ls.calleeName(funcExpr)
			_ = fe
		} else {
			// Fallback
			fnName = ls.calleeName(funcExpr)
		}

		// Create result list
		resultList := ls.b.FreshTemp("filter_result_list")
		ls.b.Emit(&hir.Call{Dst: resultList, Fn: "list_new", Args: nil})

		// Get length of source list
		srcLen := ls.b.FreshTemp("src_len")
		ls.b.Emit(&hir.Call{Dst: srcLen, Fn: "list_len", Args: []hir.Value{receiver}})

		// Create index pointer
		idxPtr := ls.b.FreshTemp("idx_ptr")
		ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
		ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

		// Create condition block
		condBlk := ls.b.NewBlock("filter_cond")
		oldCur := ls.b.Block()

		ls.b.SetBlock(condBlk)
		idxVal := ls.b.FreshTemp("filter_idx")
		ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
		condTemp := ls.b.FreshTemp("filter_cond_val")
		ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: srcLen, Type: "i1"})
		ls.b.SetBlock(oldCur)

		// Create body block
		bodyBlk := ls.b.NewBlock("filter_body")
		ls.b.SetBlock(bodyBlk)

		// Load current index
		idxBody := ls.b.FreshTemp("filter_idx_body")
		ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

		// Get element from source list
		elem := ls.b.FreshTemp("filter_elem")
		ls.b.Emit(&hir.Call{Dst: elem, Fn: "list_get", Args: []hir.Value{receiver, idxBody}, Type: "ptr"})

		// Call the predicate function
		predResult := ls.b.FreshTemp("pred_result")
		ls.b.Emit(&hir.Call{Dst: predResult, Fn: fnName, Args: []hir.Value{elem}, Type: "i1"})

		// Create conditional append block
		appendBlk := ls.b.NewBlock("filter_append")
		ls.b.SetBlock(appendBlk)
		elemPtr := ls.b.FreshTemp("elem_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})
		ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{resultList, elemPtr, hir.ConstInt{Text: "0", Type: "i32"}}})

		// Emit conditional in body block
		ls.b.SetBlock(bodyBlk)
		ls.b.Emit(&hir.If{Cond: predResult, Then: appendBlk, Else: nil})

		// Increment index (after conditional)
		nextIdx := ls.b.FreshTemp("next_idx")
		ls.b.Emit(&hir.BinaryOp{Dst: nextIdx, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
		ls.b.Emit(&hir.Store{Dst: idxPtr, Val: nextIdx})

		// Emit while loop from original block
		ls.b.SetBlock(oldCur)
		ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})

		return resultList
	}

	return nil
}
