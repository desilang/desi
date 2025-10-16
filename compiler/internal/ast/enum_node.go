package ast

import "github.com/desilang/desi/compiler/internal/diag"

type EnumVariantDecl struct {
	Name Ident
	Type *TypeName // optional payload
	Span diag.Span
}

type EnumDecl struct {
	Pub        bool
	Name       Ident
	Variants   []*EnumVariantDecl
	Decorators []*Decorator
	Doc        *StrLit // optional docstring (first stmt)
	Span       diag.Span
}

func (*EnumDecl) isDecl()             {}
func (d *EnumDecl) SpanOf() diag.Span { return d.Span }
