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
