package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerSetLit(x *ast.SetLit, t *types.Set) hir.Value {
	// 1. Allocate new set with to_str function
	dict := ls.b.FreshTemp("set")

	// Determine to_str function for set elements
	var toStrFunc hir.Value = hir.ConstNull{}
	if t != nil {
		toStrFunc = resolveToStrFunc(t.Elem)
	}

	ls.b.Emit(&hir.Call{Dst: dict, Fn: "set_new", Args: []hir.Value{toStrFunc}})

	// 2. Insert elements
	for _, elem := range x.Elems {
		keyVal := ls.lowerExpr(elem)
		// Set takes ownership of stored pointer elements
		if id, ok := elem.(*ast.Ident); ok {
			ls.cur().moved[id.Name] = true
		}
		ls.consumeTemp(keyVal) // set stores the raw pointer bits
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
		// Set takes ownership of stored pointer elements
		if id, ok := args[0].(*ast.Ident); ok {
			ls.cur().moved[id.Name] = true
		}
		ls.consumeTemp(elem) // set stores the raw pointer bits
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

	case "to_str":
		// to_str() -> str
		res := ls.b.FreshTemp("str")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "set_to_str", Args: []hir.Value{receiver}, Type: "ptr"})
		return res
	}

	return nil
}
