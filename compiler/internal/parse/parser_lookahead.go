package parse

import "github.com/desilang/desi/compiler/internal/token"
import "github.com/desilang/desi/compiler/internal/lex"

// afterMatchingParenIs returns true if, starting with p.cur == '(',
// the token immediately following the matching ')' is `want`.
func (p *Parser) afterMatchingParenIs(want token.Token) bool {
	// Index 0 refers to p.peek; further indices into p.ahead.
	balance := 1
	i := 0
	for {
		tok := p.peek
		if i > 0 {
			// Ensure we have at least i items in ahead.
			if !p.ensureAhead(i) {
				return false
			}
			tok = p.ahead[i-1]
		}
		switch tok.Tok {
		case token.LPAREN:
			balance++
		case token.RPAREN:
			balance--
			if balance == 0 {
				// The token after this ')' is at index i+1.
				var next lex.Item
				if i == -1 {
					next = p.peek
				} else {
					if !p.ensureAhead(i + 1) {
						return false
					}
					next = p.ahead[i]
				}
				return next.Tok == want
			}
		case token.EOF:
			return false
		}
		i++
	}
}

// ensureAhead ensures p.ahead has at least n items by fetching from scanner.
func (p *Parser) ensureAhead(n int) bool {
	for len(p.ahead) < n {
		p.ahead = append(p.ahead, p.sc.Next())
	}
	return true
}
