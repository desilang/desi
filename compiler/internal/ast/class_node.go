package ast

import "github.com/desilang/desi/compiler/internal/diag"

type FieldDecl struct {
	Pub  bool
	Mut  bool // mutable field (pub mut field: Type)
	Name Ident
	Type *TypeName // optional
	Span diag.Span
}

// Make FieldDecl satisfy ast.Node (needed by the pretty-printer).
func (f *FieldDecl) SpanOf() diag.Span { return f.Span }

type ClassDecl struct {
	Pub          bool
	Name         Ident
	TypeParams   []Ident     // e.g., [T] for class Container<T>
	Bases        []*TypeName // optional base classes
	Methods      []*FuncDecl
	Fields       []*FieldDecl
	Constants    []*ClassConstDecl
	StaticFields []*ClassStaticDecl
	Nested       []*ClassDecl
	Decorators   []*Decorator
	Doc          *StrLit // optional docstring (first stmt)
	Span         diag.Span
}

func (*ClassDecl) isDecl()             {}
func (*ClassDecl) isStmt()             {}
func (d *ClassDecl) SpanOf() diag.Span { return d.Span }

// ClassConstDecl represents a class-level constant declaration:
// pub const NAME: Type = Value
type ClassConstDecl struct {
	Name  Ident
	Type  *TypeName
	Value Expr
	IsPub bool
	Span  diag.Span
}

func (*ClassConstDecl) isStmt()             {}
func (x *ClassConstDecl) SpanOf() diag.Span { return x.Span }

type ClassStaticDecl struct {
	Name  Ident
	Type  *TypeName
	Value Expr
	IsPub bool
	IsMut bool
	Span  diag.Span
}

func (*ClassStaticDecl) isStmt()             {}
func (x *ClassStaticDecl) SpanOf() diag.Span { return x.Span }
