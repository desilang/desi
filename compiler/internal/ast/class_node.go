package ast

import "github.com/desilang/desi/compiler/internal/diag"

type FieldDecl struct {
	Name Ident
	Type *TypeName // optional
	Span diag.Span
}

type ClassDecl struct {
	Name       Ident
	Bases      []*TypeName // NEW: optional base classes, supports multiple
	Methods    []*FuncDecl
	Fields     []*FieldDecl
	Nested     []*ClassDecl
	Decorators []*Decorator
	Doc        *StrLit // optional docstring (first stmt)
	Span       diag.Span
}

func (*ClassDecl) isDecl()             {}
func (d *ClassDecl) SpanOf() diag.Span { return d.Span }
