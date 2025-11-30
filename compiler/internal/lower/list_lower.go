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
		res64 := ls.b.FreshTemp("len64")
		ls.b.Emit(&hir.Call{Dst: res64, Fn: "list_len", Args: []hir.Value{receiver}})

		res := ls.b.FreshTemp("len")
		ls.b.Emit(&hir.Cast{Dst: res, Src: res64, Type: "i32"})
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

	case "insert":
		// insert(index, elem)
		index := ls.lowerExpr(args[0])
		elem := ls.lowerExpr(args[1])
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
	}

	return nil
}
