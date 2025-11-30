package check

import (
	"strconv"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) typIndexExpr(x *ast.IndexExpr) types.T {
	lhs := c.typ(x.X)
	if lhs == nil {
		return nil
	}

	switch t := lhs.(type) {
	case *types.Tuple:
		// Tuple indexing requires constant integer index
		idx, ok := x.Idx.(*ast.IntLit)
		if !ok {
			c.add(diagAt("DTE0006", x.Idx.SpanOf(), "tuple index must be a constant integer"))
			return nil
		}

		i, err := strconv.Atoi(idx.Text)
		if err != nil {
			c.add(diagAt("DTE0006", x.Idx.SpanOf(), "invalid integer index"))
			return nil
		}

		if i < 0 || i >= len(t.Elems) {
			c.add(diagAt("DTE0007", x.Idx.SpanOf(), "tuple index out of bounds"))
			return nil
		}

		res := t.Elems[i]
		c.info.Types[x] = res
		return res

	case *types.List:
		// List indexing: l[i] -> T
		// Index must be int
		idxType := c.typ(x.Idx)
		if !types.Equal(idxType, types.Int) {
			c.add(diagAt("DTE0006", x.Idx.SpanOf(), "list index must be an integer"))
			return nil
		}
		c.info.Types[x] = t.Elem
		return t.Elem

	case *types.Dict:
		// Dict indexing: d[k] -> V
		// Key must match KeyType
		keyType := c.typ(x.Idx)
		if !types.Assignable(t.Key, keyType) {
			c.add(diagAt("DTE0006", x.Idx.SpanOf(), "dict key type mismatch"))
			return nil
		}
		c.info.Types[x] = t.Val
		return t.Val

	case *types.Enum, *types.Struct:
		// Generic type instantiation: Option[int], Result[str, str]
		var typeParams []types.TypeParam
		var baseName string

		if et, ok := t.(*types.Enum); ok {
			typeParams = et.TypeParams
			baseName = et.Name
		} else if st, ok := t.(*types.Struct); ok {
			typeParams = st.TypeParams
			baseName = st.Name
		}

		if len(typeParams) == 0 {
			c.add(diagAt("DTE0005", x.Span, "type '"+baseName+"' is not generic"))
			return nil
		}

		// Extract arguments
		var args []ast.Expr
		if tup, ok := x.Idx.(*ast.TupleLit); ok {
			args = tup.Elems
		} else {
			args = []ast.Expr{x.Idx}
		}

		if len(args) != len(typeParams) {
			c.add(diagAt("DTE0046", x.Idx.SpanOf(), "generic type argument count mismatch"))
			return nil
		}

		// Resolve arguments to types
		var typeArgs []types.T
		for _, arg := range args {
			tArg := c.resolveTypeExpr(arg)
			if tArg == nil {
				c.add(diagAt("DTE0004", arg.SpanOf(), "invalid type argument"))
				return nil
			}
			typeArgs = append(typeArgs, tArg)
		}

		// Create Generic instance
		gen := &types.Generic{
			Base: t,
			Args: typeArgs,
		}
		c.info.Types[x] = gen
		return gen
	}

	// Handle Basic types (Str)
	if types.Equal(lhs, types.Str) {
		// String indexing: s[i] -> str (char)
		idxType := c.typ(x.Idx)
		if !types.Equal(idxType, types.Int) {
			c.add(diagAt("DTE0006", x.Idx.SpanOf(), "string index must be an integer"))
			return nil
		}
		c.info.Types[x] = types.Str
		return types.Str
	}

	c.add(diagAt("DTE0005", x.Span, "type not indexable"))
	return nil
}

// resolveTypeExpr resolves an expression to a type (for generic arguments)
func (c *checker) resolveTypeExpr(e ast.Expr) types.T {
	switch x := e.(type) {
	case *ast.Ident:
		if sym := c.scope.Lookup(x.Name); sym != nil && sym.Kind == SymType {
			return sym.Type
		}
		// Fallback: check for built-in types (int, str, bool, etc.)
		if t, ok := types.FromName(x.Name); ok {
			return t
		}
	case *ast.IndexExpr:
		// Recursive generic instantiation: Option[Option[int]]
		return c.typIndexExpr(x)
	case *ast.FieldExpr:
		// Module type access: std.io.File
		// TODO: Implement if needed
	}
	return nil
}
