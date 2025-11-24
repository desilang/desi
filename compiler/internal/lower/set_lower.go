package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerSetLit(x *ast.SetLit, t *types.Set) hir.Value {
	// 1. Allocate new set
	// set_new() -> ptr
	dict := ls.b.FreshTemp("set")
	ls.b.Emit(&hir.Call{Dst: dict, Fn: "set_new", Args: []hir.Value{}})

	// 2. Insert elements
	for _, elem := range x.Elems {
		keyVal := ls.lowerExpr(elem)
		ls.b.Emit(&hir.Call{Fn: "set_add", Args: []hir.Value{dict, keyVal}})
	}

	return dict
}

func (ls *lowerState) lowerSetMethod(fe *ast.FieldExpr, args []ast.Expr, setType *types.Set) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name

	switch method {
	case "add":
		// add(elem)
		elem := ls.lowerExpr(args[0])
		ls.b.Emit(&hir.Call{Fn: "set_add", Args: []hir.Value{receiver, elem}})
		return nil

	case "remove":
		// remove(elem)
		elem := ls.lowerExpr(args[0])
		ls.b.Emit(&hir.Call{Fn: "set_remove", Args: []hir.Value{receiver, elem}})
		return nil

	case "contains":
		// contains(elem) -> bool
		elem := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("has")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "set_contains", Args: []hir.Value{receiver, elem}})
		return res

	case "clear":
		// clear()
		ls.b.Emit(&hir.Call{Fn: "set_clear", Args: []hir.Value{receiver}})
		return nil

	case "free":
		// free()
		ls.b.Emit(&hir.Call{Fn: "set_free", Args: []hir.Value{receiver}})
		return nil

	case "union":
		// union(other) -> set[T]
		other := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("union")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "set_union", Args: []hir.Value{receiver, other}})
		return res

	case "intersection":
		// intersection(other) -> set[T]
		other := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("intersection")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "set_intersection", Args: []hir.Value{receiver, other}})
		return res

	case "difference":
		// difference(other) -> set[T]
		other := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("difference")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "set_difference", Args: []hir.Value{receiver, other}})
		return res
	}

	return nil
}
