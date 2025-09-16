package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

// posFrom converts a lexer.Token into an ast.Pos using the token's start location.
func posFrom(t lexer.Token) ast.Pos {
	return ast.Pos{Line: t.Line, Col: t.Col}
}

// spanTok makes a Span from two tokens (inclusive start, inclusive end).
// We treat End as the end token's starting position plus 1 column, which is
// good enough for single-token nodes. Multi-token nodes can use the closing token.
func spanTok(start, end lexer.Token) ast.Span {
	// Note: we don't currently know the token width; stage-0 uses start cols.
	// We'll set End to the end token start + 1, which is a reasonable pointer.
	return ast.Span{
		Start: posFrom(start),
		End:   ast.Pos{Line: end.Line, Col: end.Col + 1},
	}
}
