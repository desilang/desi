package parse

import (
	"reflect"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// lastSpan returns n.SpanOf() or a fallback if n is nil.
//
// Be careful: an interface holding a typed-nil pointer (e.g. (*ast.Ident)(nil))
// is NOT == nil. We must detect that and avoid calling methods on a nil receiver.
func lastSpan(n ast.Node, fallback diag.Span) diag.Span {
	if n == nil {
		return fallback
	}
	rv := reflect.ValueOf(n)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return fallback
	}
	return n.SpanOf()
}
