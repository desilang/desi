package ast

import "github.com/desilang/desi/compiler/internal/diag"

// Decorator represents '@name(...)' applied to the next declaration.
type Decorator struct {
	Name   Ident           // decorator identifier (dotted allowed in parse; kept as Ident here)
	Args   []Expr          // positional args
	KwArgs map[string]Expr // keyword args (e.g., safe=true, c_name="...")
	Span   diag.Span
}

func (d *Decorator) SpanOf() diag.Span { return d.Span }
