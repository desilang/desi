package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerDictMethod(fe *ast.FieldExpr, args []ast.Expr, dictType *types.Dict) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name

	switch method {
	case "get":
		// get(key, default)
		key := ls.lowerExpr(args[0])
		defVal := ls.lowerExpr(args[1])

		// Spill default value to stack to pass as pointer
		defPtr := ls.b.FreshTemp("def_ptr")
		// Use i64 for generic value slot (Tier-0)
		ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: defPtr})
		ls.b.Emit(&hir.Store{Dst: defPtr, Val: defVal})

		resPtr := ls.b.FreshTemp("res_ptr")
		ls.b.Emit(&hir.Call{Dst: resPtr, Fn: "dict_get", Args: []hir.Value{receiver, key, defPtr}})

		// Load result from pointer
		valDst := ls.b.FreshTemp("val")
		ls.b.Emit(&hir.Load{Type: "i64", Src: resPtr, Dst: valDst})
		return valDst

	case "insert":
		// insert(key, value)
		key := ls.lowerExpr(args[0])
		val := ls.lowerExpr(args[1])

		// Cast to i64 to ensure we store 8 bytes (Tier-0 universal value size)
		val64 := ls.b.FreshTemp("val64")
		ls.b.Emit(&hir.Cast{Dst: val64, Src: val, Type: "i64"})

		// Spill value to stack to pass as pointer
		valPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: valPtr})
		ls.b.Emit(&hir.Store{Dst: valPtr, Val: val64})

		ls.b.Emit(&hir.Call{Fn: "dict_insert", Args: []hir.Value{receiver, key, valPtr}})
		return nil

	case "has_key":
		// has_key(key)
		key := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("has")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "dict_has_key", Args: []hir.Value{receiver, key}, Type: "i1"})
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
