package ast

import "github.com/desilang/desi/compiler/internal/diag"

// TraitDecl represents a 'trait Name: ...' declaration.
type TraitDecl struct {
	Pub        bool
	Name       Ident
	Methods    []*FuncDecl // signatures only (Body=nil)
	Decorators []*Decorator
	Doc        *StrLit
	Span       diag.Span
}

func (d *TraitDecl) isDecl()           {}
func (d *TraitDecl) SpanOf() diag.Span { return d.Span }

// ImplDecl represents an 'impl Trait for Type: ...' block.
type ImplDecl struct {
	Trait   *TypeName // The trait being implemented
	ForType *TypeName // The type implementing the trait
	Methods []*FuncDecl
	Span    diag.Span
}

func (d *ImplDecl) isDecl()           {}
func (d *ImplDecl) SpanOf() diag.Span { return d.Span }
