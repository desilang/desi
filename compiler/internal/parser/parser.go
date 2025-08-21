package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
)

// Parser scaffolding; real implementation will hook the lexer and build AST.
type Parser struct{}

func New() *Parser { return &Parser{} }

func (p *Parser) ParseFile(src string) (*ast.File, error) {
	// TODO: connect lexer, produce AST
	return &ast.File{Decls: []ast.Decl{}}, nil
}
