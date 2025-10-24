package ast

import "github.com/desilang/desi/compiler/internal/diag"

// ImportStmt models:  import dotted.name [as alias]
type ImportStmt struct {
	// Path is the dotted module path split into segments: ["foo","bar"] for "foo.bar".
	Path  []string
	Alias *Ident    // optional local binding name; if nil, last Path segment is bound
	Span  diag.Span // full statement span
}

func (*ImportStmt) isStmt()        {}
func (s *ImportStmt) SpanOf() Span { return s.Span }

// FromImportItem is one imported item in:  from pkg import a [as x], b, ...
type FromImportItem struct {
	Name  Ident     // the imported name (value or direct submodule)
	Alias *Ident    // optional alias
	Span  diag.Span // span covering the item (name..alias if present)
}

// FromImportStmt models:  from dotted.name import a [as x], b, ...
type FromImportStmt struct {
	Path  []string // dotted module path split into segments
	Items []FromImportItem
	Span  diag.Span
}

func (*FromImportStmt) isStmt()        {}
func (s *FromImportStmt) SpanOf() Span { return s.Span }
