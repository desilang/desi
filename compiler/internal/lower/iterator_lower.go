package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// lowerListIterMethod handles method calls on ListIter types: collect, first
func (ls *lowerState) lowerListIterMethod(fe *ast.FieldExpr, args []ast.Expr, iterType *types.ListIter) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	methodName := fe.Name.Name

	switch methodName {
	case "collect":
		// collect() -> list[T]
		// Call: list_iter_collect(iter_ptr) -> DesiList*
		// The C runtime function iterates and builds the list
		result := ls.b.FreshTemp("collected")
		typeTag := ls.getIterTypeTag(iterType.Elem)
		ls.b.Emit(&hir.Call{Dst: result, Fn: "list_iter_collect", Args: []hir.Value{receiver, typeTag}, Type: "ptr"})
		return result

	case "first":
		// first() -> Option[T]  (returns element or NULL)
		elem := ls.b.FreshTemp("first_elem")
		ls.b.Emit(&hir.Call{Dst: elem, Fn: "list_iter_next", Args: []hir.Value{receiver}, Type: "ptr"})
		return elem

	default:
		// Unsupported methods
		return nil
	}
}

// getIterTypeTag returns the type tag for list element types
func (ls *lowerState) getIterTypeTag(t types.T) hir.Value {
	switch t {
	case types.Int:
		return hir.ConstInt{Text: "0"}
	case types.Str:
		return hir.ConstInt{Text: "1"}
	case types.Bool:
		return hir.ConstInt{Text: "2"}
	default:
		return hir.ConstInt{Text: "3"} // Unknown/custom type
	}
}
