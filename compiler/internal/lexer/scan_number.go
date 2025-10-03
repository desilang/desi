package lexer

import "unicode"

// scanNumber is kept for compatibility (hex/bin/decimal INT only).
// Prefer scanNumberWithOptionalFractionAndExp from Next().
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

// scanNumberWithOptionalFractionAndExp scans a decimal integer OR float.
// Supported floats:
//
//	digits '.' digits [exponent]?  OR  digits exponent
//
// exponent := ('e'|'E') ['+'|'-'] digits
// We do NOT accept leading-dot (.5) or trailing-dot (1.) forms (by design).
func (lx *Lexer) scanNumberWithOptionalFractionAndExp() (lex string, kind TokKind) {
	start := lx.i

	// Hex/bin prefixes → always INT
	if ch, ok := lx.peek(); ok && ch == '0' {
		// Lookahead without consuming: 0x / 0b?
		if lx.i+1 < len(lx.src) {
			n1 := lx.src[lx.i+1]
			if n1 == 'x' || n1 == 'X' {
				// consume 0x and hex digits
				lx.advance() // '0'
				lx.advance() // 'x'
				for {
					r, ok := lx.peek()
					if !ok || !(unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
						break
					}
					lx.advance()
				}
				return string(lx.src[start:lx.i]), TokInt
			}
			if n1 == 'b' || n1 == 'B' {
				// consume 0b and binary digits
				lx.advance() // '0'
				lx.advance() // 'b'
				for {
					r, ok := lx.peek()
					if !ok || !(r == '0' || r == '1') {
						break
					}
					lx.advance()
				}
				return string(lx.src[start:lx.i]), TokInt
			}
		}
	}

	// Consume leading decimal digits
	for {
		r, ok := lx.peek()
		if !ok || !unicode.IsDigit(r) {
			break
		}
		lx.advance()
	}

	isFloat := false

	// Fractional part: only if we see '.' followed by a digit
	if lx.i < len(lx.src) && lx.src[lx.i] == '.' {
		if lx.i+1 < len(lx.src) && unicode.IsDigit(lx.src[lx.i+1]) {
			isFloat = true
			lx.advance() // '.'
			for {
				r, ok := lx.peek()
				if !ok || !unicode.IsDigit(r) {
					break
				}
				lx.advance()
			}
		}
	}

	// Exponent part (optional). Only consume if it's valid as a whole.
	// Lookahead first so we don't over-consume on invalid "1e".
	if lx.i < len(lx.src) && (lx.src[lx.i] == 'e' || lx.src[lx.i] == 'E') {
		j := lx.i + 1
		if j < len(lx.src) && (lx.src[j] == '+' || lx.src[j] == '-') {
			j++
		}
		if j < len(lx.src) && unicode.IsDigit(lx.src[j]) {
			// Valid exponent → consume it.
			isFloat = true
			lx.advance() // 'e'/'E'
			if ch, ok := lx.peek(); ok && (ch == '+' || ch == '-') {
				lx.advance()
			}
			for {
				r, ok := lx.peek()
				if !ok || !unicode.IsDigit(r) {
					break
				}
				lx.advance()
			}
		}
	}

	if isFloat {
		return string(lx.src[start:lx.i]), TokFloat
	}
	return string(lx.src[start:lx.i]), TokInt
}
