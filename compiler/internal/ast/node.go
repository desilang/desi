package ast

/*** NODES (roots, common interfaces) ***/

type Node interface{ node() }

// File is a compilation unit.
type File struct {
	Pkg         *PackageDecl
	Imports     []ImportDecl
	FromImports []FromImportDecl
	Decls       []Decl
}

func (File) node() {}

type PackageDecl struct {
	Name string
	Span Span // optional
}

func (PackageDecl) node() {}

type ImportDecl struct {
	Path    string   // e.g. "std.io"
	Aliases []string // reserved for future use
	Span    Span     // optional
	As      string
}

func (ImportDecl) node() {}

/*** NEW: from-imports with optional aliasing ***/

type FromImportDecl struct {
	Module string       // dotted module path, e.g. "util.math"
	Items  []ImportItem // items imported from the module
	Span   Span         // optional
}

func (FromImportDecl) node() {}

type ImportItem struct {
	Name string // source name in the module (e.g., "sqrt")
	As   string // local alias; empty means import as original name
	Span Span   // optional
}

type Decl interface {
	Node
	decl()
}
