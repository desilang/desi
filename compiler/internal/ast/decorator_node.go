package ast

import "github.com/desilang/desi/compiler/internal/diag"

// Decorator represents '@name(...)' applied to the next declaration.
type Decorator struct {
	Name Ident  // decorator identifier (dotted allowed in parse; kept as Ident here)
	Args []Expr // optional positional args
	Span diag.Span
}

func (d *Decorator) SpanOf() diag.Span { return d.Span }
