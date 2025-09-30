package lexer

// handle beginning-of-line: compute indentation and queue INDENT/DEDENT/skip blanks.
func (lx *Lexer) handleBOL() {
	for lx.bol {
		// EOF: unwind any remaining indents
		if lx.atEOF() {
			for len(lx.indents) > 1 {
				lx.indents = lx.indents[:len(lx.indents)-1]
				lx.enqueue(lx.make(TokDedent, "", lx.line, lx.col))
			}
			lx.bol = false
			return
		}

		// Count indentation (spaces/tabs) but don't consume newline yet
		width := 0
		for {
			ch, ok := lx.peek()
			if !ok {
				break
			}
			if ch == ' ' {
				width++
				lx.advance()
				continue
			}
			if ch == '\t' {
				width += 4 // Stage-0: TAB = 4 spaces
				lx.advance()
				continue
			}
			break
		}

		// Blank or comment-only line? Consume to newline and continue at BOL.
		if ch, ok := lx.peek(); !ok {
			// EOF after spaces: just unwind in next loop
		} else if ch == '\n' {
			lx.advance() // eat newline
			// keep bol=true; skip emitting NEWLINE for blank lines
			continue
		} else if ch == '#' {
			// consume comment to end-of-line
			for {
				ch, ok := lx.peek()
				if !ok || ch == '\n' {
					break
				}
				lx.advance()
			}
			if lx.match('\n') {
				// comment-only line: skip NEWLINE
				continue
			}
			// fallthrough if EOF
		}

		// Compare indentation with top of stack
		top := lx.indents[len(lx.indents)-1]
		if width > top {
			lx.indents = append(lx.indents, width)
			lx.enqueue(lx.make(TokIndent, "", lx.line, lx.col))
		} else if width < top {
			for width < top && len(lx.indents) > 1 {
				lx.indents = lx.indents[:len(lx.indents)-1]
				top = lx.indents[len(lx.indents)-1]
				lx.enqueue(lx.make(TokDedent, "", lx.line, lx.col))
			}
			// If width != top here, it's a malformed indent; Stage-0: ignore extra for now.
		}
		lx.bol = false
		// We leave lx.i at first non-space char to be lexed by Next()
		if len(lx.pending) > 0 {
			return
		}
		// Otherwise we proceed to lex the token on this line.
		return
	}
}
