package ast

// Node is implemented by all AST nodes.
type Node interface{ node() }

// File is a compilation unit.
type File struct {
	Decls []Decl
}

func (File) node() {}

// Decl is a top-level declaration.
type Decl interface {
	Node
	decl()
}

type FuncDecl struct {
	Name string
	// TODO: params, return type, body
}

func (FuncDecl) node() {}
func (FuncDecl) decl() {}
