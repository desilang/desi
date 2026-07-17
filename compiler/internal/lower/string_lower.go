package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// lowerStringMethod handles str method calls like split, join, replace
func (ls *lowerState) lowerStringMethod(fe *ast.FieldExpr, args []ast.Expr) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name

	switch method {
	case "split":
		// split(delim) -> list<str>
		if len(args) < 1 {
			return nil
		}
		delim := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("split_result")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "string_split", Args: []hir.Value{receiver, delim}, Type: "ptr"})
		return res

	case "replace":
		// replace(old, new) -> str
		if len(args) < 2 {
			return nil
		}
		oldStr := ls.lowerExpr(args[0])
		newStr := ls.lowerExpr(args[1])
		res := ls.b.FreshTemp("replace_result")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "string_replace", Args: []hir.Value{receiver, oldStr, newStr}, Type: "ptr"})
		// string_replace mallocs — track so a transient result is freed.
		ls.addTempDrop(res.Name)
		return res
	}

	return nil
}

// lowerListJoin handles list<str>.join(delim) -> str
func (ls *lowerState) lowerListJoin(fe *ast.FieldExpr, args []ast.Expr, listType *types.List) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name

	if method == "join" && listType.Elem == types.Str {
		// join(delim) -> str
		if len(args) < 1 {
			return nil
		}
		delim := ls.lowerExpr(args[0])
		res := ls.b.FreshTemp("join_result")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "string_join", Args: []hir.Value{receiver, delim}, Type: "ptr"})
		// string_join mallocs — track so a transient result is freed.
		ls.addTempDrop(res.Name)
		return res
	}

	return nil
}
