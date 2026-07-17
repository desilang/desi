package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// Helper to prepare key arguments based on key type
// Returns keyInt, keyStr, keyFloat, keyPtr
func (ls *lowerState) prepareKeyArgs(keyExpr ast.Expr, keyType types.T) (keyInt hir.Value, keyStr hir.Value, keyFloat hir.Value, keyPtr hir.Value) {
	keyVal := ls.lowerExpr(keyExpr)

	keyInt = hir.ConstInt{Text: "0", Type: "i64"}
	keyStr = hir.ConstNull{}
	keyFloat = hir.ConstFloat{Text: "0.0"}
	keyPtr = hir.ConstNull{}

	isStrKey := types.Equal(keyType, types.Str)
	isFloatKey := keyType == types.Float || keyType == types.F32 || keyType == types.F64

	// Check for any custom key type (class, struct, enum, list, set)
	isCustomKey := false
	switch keyType.(type) {
	case *types.Class, *types.Struct, *types.Enum, *types.List, *types.Set:
		isCustomKey = true
	}

	if isStrKey {
		keyStr = keyVal
	} else if isFloatKey {
		keyFloat = keyVal
	} else if isCustomKey {
		keyPtr = keyVal
	} else {
		// int or bool - cast to i64
		keyInt64 := ls.b.FreshTemp("key_i64")
		ls.b.Emit(&hir.Cast{Dst: keyInt64, Src: keyVal, Type: "i64"})
		keyInt = keyInt64
	}

	return keyInt, keyStr, keyFloat, keyPtr
}

func (ls *lowerState) lowerDictMethod(fe *ast.FieldExpr, args []ast.Expr, dictType *types.Dict) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name
	keyType := dictType.Key

	switch method {
	case "get":
		// get(key, default)
		keyInt, keyStr, keyFloat, keyPtr := ls.prepareKeyArgs(args[0], keyType)
		defVal := ls.lowerExpr(args[1])

		// Spill default value to stack to pass as pointer
		defPtr := ls.b.FreshTemp("def_ptr")
		ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: defPtr})
		ls.b.Emit(&hir.Store{Dst: defPtr, Val: defVal})

		resPtr := ls.b.FreshTemp("res_ptr")
		// dict_get(dict, key_int, key_str, key_float, key_ptr, default)
		ls.b.Emit(&hir.Call{Dst: resPtr, Fn: "dict_get", Args: []hir.Value{receiver, keyInt, keyStr, keyFloat, keyPtr, defPtr}})

		// Load result from pointer
		valDst := ls.b.FreshTemp("val")
		ls.b.Emit(&hir.Load{Type: "i64", Src: resPtr, Dst: valDst})

		// For pointer value types (str, class, list, set, dict), convert i64 back to ptr
		valType := dictType.Val
		needsPtrCast := types.Equal(valType, types.Str)
		switch valType.(type) {
		case *types.Class, *types.List, *types.Set, *types.Dict, *types.Struct:
			needsPtrCast = true
		}
		if needsPtrCast {
			ptrDst := ls.b.FreshTemp("val_ptr")
			ls.b.Emit(&hir.Cast{Src: valDst, Dst: ptrDst, Type: "ptr"})
			return ptrDst
		}
		return valDst

	case "setdefault":
		// setdefault(key, default) -> Value
		keyInt, keyStr, keyFloat, keyPtr := ls.prepareKeyArgs(args[0], keyType)
		defVal := ls.lowerExpr(args[1])
		// The default may be retained as the stored value — consume temps.
		ls.consumeTemp(defVal)

		// Get default value type
		var valType types.T
		if ls.info != nil {
			valType = ls.info.Types[args[1]]
		}

		// Similar to insert, handle float bitcast and store to stack
		isFloat := valType == types.Float || valType == types.F32 || valType == types.F64

		defPtr := ls.b.FreshTemp("def_ptr")
		if isFloat {
			val64 := ls.b.FreshTemp("val64")
			ls.b.Emit(&hir.BitCast{Val: defVal, Dst: val64, Type: "i64"})
			ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: defPtr})
			ls.b.Emit(&hir.Store{Dst: defPtr, Val: val64})
		} else {
			// For non-float, we can store directly if size matches, but for safety lets cast to i64 then store
			// Primitives are stored as i64 in dict.
			// Wait, for 'get' above we just stared.
			// dict_get expects default_val as pointer.
			// dict_insert expects value as pointer AND type tag.
			// dict_setdefault needs both.
			val64 := ls.b.FreshTemp("val64")
			ls.b.Emit(&hir.Cast{Dst: val64, Src: defVal, Type: "i64"})
			ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: defPtr})
			ls.b.Emit(&hir.Store{Dst: defPtr, Val: val64})
		}

		// Determine type tag
		var typeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
		if ls.info != nil {
			typeTag = getTypeTag(ls.info.Types[args[1]])
		}

		resPtr := ls.b.FreshTemp("res_ptr")
		// dict_setdefault(dict, key_int, key_str, key_float, key_ptr, default_ptr, type_tag)
		ls.b.Emit(&hir.Call{Dst: resPtr, Fn: "dict_setdefault", Args: []hir.Value{receiver, keyInt, keyStr, keyFloat, keyPtr, defPtr, typeTag}, Type: "ptr"})

		// Load result
		valDst := ls.b.FreshTemp("val")
		ls.b.Emit(&hir.Load{Type: "i64", Src: resPtr, Dst: valDst})

		// Cast back if needed (copy-paste from get)
		dictValType := dictType.Val
		needsPtrCast := types.Equal(dictValType, types.Str)
		switch dictValType.(type) {
		case *types.Class, *types.List, *types.Set, *types.Dict, *types.Struct:
			needsPtrCast = true
		}
		if needsPtrCast {
			ptrDst := ls.b.FreshTemp("val_ptr")
			ls.b.Emit(&hir.Cast{Src: valDst, Dst: ptrDst, Type: "ptr"})
			return ptrDst
		}
		return valDst

	case "insert":
		// insert(key, value)
		// Keys are strdup'd by dict_insert (temp keys may be freed), but
		// values are stored as raw pointer slots — consume value temps.
		keyInt, keyStr, keyFloat, keyPtr := ls.prepareKeyArgs(args[0], keyType)
		val := ls.lowerExpr(args[1])
		ls.consumeTemp(val)

		// Get value type from type info
		var valType types.T
		if ls.info != nil {
			valType = ls.info.Types[args[1]]
		}

		// Check if value is a float type - need BitCast to preserve bits
		isFloat := valType == types.Float || valType == types.F32 || valType == types.F64

		// Widen the value to an i64 slot and pass it BY VALUE — an alloca
		// spill here would allocate stack per loop iteration (LLVM only
		// reclaims allocas on function return) and overflow in long
		// insert loops.
		val64 := ls.b.FreshTemp("val64")
		if isFloat {
			ls.b.Emit(&hir.BitCast{Val: val, Dst: val64, Type: "i64"})
		} else {
			ls.b.Emit(&hir.Cast{Dst: val64, Src: val, Type: "i64"})
		}

		// Determine type tag
		var typeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
		if ls.info != nil {
			typeTag = getTypeTag(ls.info.Types[args[1]])
		}

		// dict_insert_val(dict, key_int, key_str, key_float, key_ptr, value, value_type_tag)
		ls.b.Emit(&hir.Call{Fn: "dict_insert_val", Args: []hir.Value{receiver, keyInt, keyStr, keyFloat, keyPtr, val64, typeTag}})
		return nil

	case "has_key":
		// has_key(key)
		keyInt, keyStr, keyFloat, keyPtr := ls.prepareKeyArgs(args[0], keyType)
		res := ls.b.FreshTemp("has")
		// dict_has_key(dict, key_int, key_str, key_float, key_ptr)
		ls.b.Emit(&hir.Call{Dst: res, Fn: "dict_has_key", Args: []hir.Value{receiver, keyInt, keyStr, keyFloat, keyPtr}, Type: "i1"})
		return res

	case "clear":
		// clear()
		ls.b.Emit(&hir.Call{Fn: "dict_clear", Args: []hir.Value{receiver}})
		return nil

	case "free":
		// free() - manual memory management helper
		ls.b.Emit(&hir.Call{Fn: "dict_free", Args: []hir.Value{receiver}})
		return nil

	case "to_str":
		// to_str() -> str
		res := ls.b.FreshTemp("str")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "dict_to_str", Args: []hir.Value{receiver}, Type: "ptr"})
		return res
	}

	return nil
}
