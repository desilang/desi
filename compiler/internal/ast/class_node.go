package ast

import "github.com/desilang/desi/compiler/internal/diag"

type FieldDecl struct {
	Pub  bool
	Name Ident
	Type *TypeName // optional
	Span diag.Span
}

// Make FieldDecl satisfy ast.Node (needed by the pretty-printer).
func (f *FieldDecl) SpanOf() diag.Span { return f.Span }

type ClassDecl struct {
	Pub        bool
	Name       Ident
	Bases      []*TypeName // optional base classes
	Methods    []*FuncDecl
	Fields     []*FieldDecl
	Nested     []*ClassDecl
	Decorators []*Decorator
	Doc        *StrLit // optional docstring (first stmt)
	Span       diag.Span
}

func (*ClassDecl) isDecl()             {}
func (d *ClassDecl) SpanOf() diag.Span { return d.Span }
