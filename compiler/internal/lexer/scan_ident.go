package lexer

import "unicode"

// ----- scanning helpers -----

func isIdentStart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r)
}
func isIdentPart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (lx *Lexer) scanIdent() string {
	start := lx.i
	for {
		r, ok := lx.peek()
		if !ok || !isIdentPart(r) {
			break
		}
		lx.advance()
	}
	return string(lx.src[start:lx.i])
}
