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
	}

	return nil
}
