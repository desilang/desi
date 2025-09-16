package ast

/*** DECLS ***/

type FuncDecl struct {
	Name   string
	Params []Param
	Ret    string // textual type for now
	Body   []Stmt

	Span Span // span of the whole decl (from 'def' to end)
}

func (FuncDecl) node() {}
func (FuncDecl) decl() {}

type Param struct {
	Name string
	Type string
	Span Span // optional: 'name: type'
}
