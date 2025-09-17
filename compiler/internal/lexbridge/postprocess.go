package lexbridge

import "github.com/desilang/desi/compiler/internal/lexer"

// Inject a NEWLINE before any DEDENT if the previous token wasn't a NEWLINE.
func injectNewlinesBeforeDedent(in []lexer.Token) []lexer.Token {
	if len(in) == 0 {
		return in
	}
	fixed := make([]lexer.Token, 0, len(in)+4)
	for _, t := range in {
		if t.Kind == lexer.TokDedent {
			if len(fixed) > 0 && fixed[len(fixed)-1].Kind != lexer.TokNewline {
				fixed = append(fixed, lexer.Token{
					Kind: lexer.TokNewline,
					Line: t.Line,
					Col:  t.Col,
				})
			}
		}
		fixed = append(fixed, t)
	}
	return fixed
}

// Ensure a trailing EOF token exists.
func ensureEOF(in []lexer.Token) []lexer.Token {
	if n := len(in); n == 0 || in[n-1].Kind != lexer.TokEOF {
		return append(in, lexer.Token{Kind: lexer.TokEOF})
	}
	return in
}
