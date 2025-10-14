package lex

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/desilang/desi/compiler/internal/token"
)

// Layoutize converts source text into a sequence of layout tokens:
//   - Emits Indent/Dedent when indentation level changes between non-blank lines
//   - Emits NL at the end of each non-blank physical line
//   - Ignores blank/comment-only lines (no NL)
//
// Indentation is measured as the count of leading spaces/tabs (no validation yet).
func Layoutize(src []byte) []token.Token {
	var out []token.Token
	sc := bufio.NewScanner(bytes.NewReader(src))

	indentStack := []int{0} // baseline
	lineIdx := 0

	for sc.Scan() {
		lineIdx++
		line := sc.Text()

		// Skip blank lines (no NL, no indent changes)
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Count leading spaces/tabs as indentation width (1 per rune)
		indent := 0
		for _, r := range line {
			if r == ' ' || r == '\t' {
				indent++
				continue
			}
			break
		}

		// Compare with current top
		cur := indentStack[len(indentStack)-1]
		if indent > cur {
			indentStack = append(indentStack, indent)
			out = append(out, token.Indent)
		} else if indent < cur {
			// Dedent until levels match (or baseline)
			for len(indentStack) > 0 && indent < indentStack[len(indentStack)-1] {
				indentStack = indentStack[:len(indentStack)-1]
				out = append(out, token.Dedent)
			}
			// If not matched, we accept "ragged" indentation silently in M0.
		}

		// End of logical line
		out = append(out, token.NL)
	}

	// Drain remaining indents at EOF
	for len(indentStack) > 1 {
		indentStack = indentStack[:len(indentStack)-1]
		out = append(out, token.Dedent)
	}

	return out
}
