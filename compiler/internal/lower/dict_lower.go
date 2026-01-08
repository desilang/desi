package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// Helper to prepare key arguments based on key type
func (ls *lowerState) prepareKeyArgs(keyExpr ast.Expr, keyType types.T) (keyInt hir.Value, keyStr hir.Value, keyFloat hir.Value) {
	keyVal := ls.lowerExpr(keyExpr)

	keyInt = hir.ConstInt{Text: "0", Type: "i64"}
	keyStr = hir.ConstNull{}
	keyFloat = hir.ConstFloat{Text: "0.0"}

	isStrKey := types.Equal(keyType, types.Str)
	isFloatKey := keyType == types.Float || keyType == types.F32 || keyType == types.F64

	if isStrKey {
		keyStr = keyVal
	} else if isFloatKey {
		keyFloat = keyVal
	} else {
		// int or bool - cast to i64
		keyInt64 := ls.b.FreshTemp("key_i64")
		ls.b.Emit(&hir.Cast{Dst: keyInt64, Src: keyVal, Type: "i64"})
		keyInt = keyInt64
	}

	return keyInt, keyStr, keyFloat
}

func (ls *lowerState) lowerDictMethod(fe *ast.FieldExpr, args []ast.Expr, dictType *types.Dict) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name
	keyType := dictType.Key

	switch method {
	case "get":
		// get(key, default)
		keyInt, keyStr, keyFloat := ls.prepareKeyArgs(args[0], keyType)
		defVal := ls.lowerExpr(args[1])

		// Spill default value to stack to pass as pointer
		defPtr := ls.b.FreshTemp("def_ptr")
		ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: defPtr})
		ls.b.Emit(&hir.Store{Dst: defPtr, Val: defVal})

		resPtr := ls.b.FreshTemp("res_ptr")
		// dict_get(dict, key_int, key_str, key_float, default)
		ls.b.Emit(&hir.Call{Dst: resPtr, Fn: "dict_get", Args: []hir.Value{receiver, keyInt, keyStr, keyFloat, defPtr}})

		// Load result from pointer
		valDst := ls.b.FreshTemp("val")
		ls.b.Emit(&hir.Load{Type: "i64", Src: resPtr, Dst: valDst})
		return valDst

	case "insert":
		// insert(key, value)
		keyInt, keyStr, keyFloat := ls.prepareKeyArgs(args[0], keyType)
		val := ls.lowerExpr(args[1])

		// Get value type from type info
		var valType types.T
		if ls.info != nil {
			valType = ls.info.Types[args[1]]
		}

		// Check if value is a float type - need BitCast to preserve bits
		isFloat := valType == types.Float || valType == types.F32 || valType == types.F64

		// Spill value to stack to pass as pointer
		valPtr := ls.b.FreshTemp("val_ptr")
		if isFloat {
			val64 := ls.b.FreshTemp("val64")
			ls.b.Emit(&hir.BitCast{Val: val, Dst: val64, Type: "i64"})
			ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: valPtr})
			ls.b.Emit(&hir.Store{Dst: valPtr, Val: val64})
		} else {
			val64 := ls.b.FreshTemp("val64")
			ls.b.Emit(&hir.Cast{Dst: val64, Src: val, Type: "i64"})
			ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: valPtr})
			ls.b.Emit(&hir.Store{Dst: valPtr, Val: val64})
		}

		// Determine type tag
		var typeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
		if ls.info != nil {
			typeTag = getTypeTag(ls.info.Types[args[1]])
		}

		// dict_insert(dict, key_int, key_str, key_float, &value, value_type_tag)
		ls.b.Emit(&hir.Call{Fn: "dict_insert", Args: []hir.Value{receiver, keyInt, keyStr, keyFloat, valPtr, typeTag}})
		return nil

	case "has_key":
		// has_key(key)
		keyInt, keyStr, keyFloat := ls.prepareKeyArgs(args[0], keyType)
		res := ls.b.FreshTemp("has")
		// dict_has_key(dict, key_int, key_str, key_float)
		ls.b.Emit(&hir.Call{Dst: res, Fn: "dict_has_key", Args: []hir.Value{receiver, keyInt, keyStr, keyFloat}, Type: "i1"})
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
