package lexer

// scanString returns the *contents* of the string literal (without quotes).
// It supports simple C-like escapes. If a closing quote is not found before
// a newline/EOF, it returns closed=false and does *not* consume the newline.
func (lx *Lexer) scanString() (val string, closed bool) {
	start := lx.i
	_ = start
	lx.advance() // consume opening "
	var out []rune
	for {
		r, ok := lx.peek()
		if !ok {
			// EOF before closing quote
			return string(out), false
		}
		if r == '\\' {
			lx.advance() // backslash
			er, ok2 := lx.peek()
			if !ok2 {
				return string(out), false
			}
			switch er {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			case '"':
				out = append(out, '"')
			case '\\':
				out = append(out, '\\')
			default:
				// unknown escape: keep literal char
				out = append(out, er)
			}
			lx.advance()
			continue
		}
		if r == '"' {
			lx.advance()
			return string(out), true
		}
		if r == '\n' {
			// newline terminates without closing quote; leave newline for caller
			return string(out), false
		}
		out = append(out, r)
		lx.advance()
	}
}
