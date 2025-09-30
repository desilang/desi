package lexer

import "unicode"

func (lx *Lexer) scanNumber() string {
	start := lx.i
	// 0x / 0b prefixes
	if ch, ok := lx.peek(); ok && ch == '0' {
		lx.advance()
		if ch2, ok2 := lx.peek(); ok2 && (ch2 == 'x' || ch2 == 'X') {
			lx.advance()
			for {
				r, ok := lx.peek()
				if !ok || !(unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
					break
				}
				lx.advance()
			}
			return string(lx.src[start:lx.i])
		}
		if ch2, ok2 := lx.peek(); ok2 && (ch2 == 'b' || ch2 == 'B') {
			lx.advance()
			for {
				r, ok := lx.peek()
				if !ok || !(r == '0' || r == '1') {
					break
				}
				lx.advance()
			}
			return string(lx.src[start:lx.i])
		}
		// fallthrough to decimal after single '0'
	}
	for {
		r, ok := lx.peek()
		if !ok || !unicode.IsDigit(r) {
			break
		}
		lx.advance()
	}
	return string(lx.src[start:lx.i])
}
