package ast

import "github.com/desilang/desi/compiler/internal/diag"

type StructDecl struct {
	Pub        bool
	Name       Ident
	TypeParams []*TypeParamNode // e.g., [T] or [T: Numeric]
	Fields     []*FieldDecl
	Decorators []*Decorator
	Doc        *StrLit // optional docstring (first stmt)
	Span       diag.Span
}

func (*StructDecl) isDecl()             {}
func (d *StructDecl) SpanOf() diag.Span { return d.Span }
