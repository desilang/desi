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
		// append(elem)
		elem := ls.lowerExpr(args[0])

		// Cast to ptr for generic storage (void*)
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})

		ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{receiver, elemPtr}})
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

		// Cast to ptr for generic storage
		elemPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: elem, Type: "ptr"})

		ls.b.Emit(&hir.Call{Fn: "list_set", Args: []hir.Value{receiver, index, elemPtr}})
		return nil

	case "len":
		// len() -> int
		res := ls.b.FreshTemp("len")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_len", Args: []hir.Value{receiver}})
		return res

	case "pop":
		// pop() -> T
		res := ls.b.FreshTemp("popped")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_pop", Args: []hir.Value{receiver}})
		return res

	case "free":
		// free()
		ls.b.Emit(&hir.Call{Fn: "list_free", Args: []hir.Value{receiver}})
		return nil
	}

	return nil
}
