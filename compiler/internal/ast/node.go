package ast

/*** NODES (roots, common interfaces) ***/

type Node interface{ node() }

// File is a compilation unit.
type File struct {
	Pkg     *PackageDecl
	Imports []ImportDecl
	Decls   []Decl
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
}

func (ImportDecl) node() {}

type Decl interface {
	Node
	decl()
}
