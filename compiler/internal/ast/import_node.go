package ast

import "github.com/desilang/desi/compiler/internal/diag"

// ImportStmt models:   import dotted.name [as alias]
type ImportStmt struct {
	Path  []string // dotted path split into segments
	Alias *Ident   // optional local alias
	Span  diag.Span
}

func (*ImportStmt) isStmt()             {}
func (s *ImportStmt) SpanOf() diag.Span { return s.Span }

// FromImportItem is a single item in:   from dotted.name import a [as x], b, ...
type FromImportItem struct {
	Name  Ident  // imported item name (identifier)
	Alias *Ident // optional local alias
	Span  diag.Span
}

// FromImportStmt models:   from dotted.name import a [as x], b, ...
//
//	or:   from dotted.name import *
type FromImportStmt struct {
	Path  []string         // dotted path split into segments
	Items []FromImportItem // imported items (empty if Star is true)
	Star  bool             // true for wildcard import (from X import *)
	Span  diag.Span
}

func (*FromImportStmt) isStmt()             {}
func (s *FromImportStmt) SpanOf() diag.Span { return s.Span }
