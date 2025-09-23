package c

import (
	"bytes"
	"strings"
)

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return string(bytes.Repeat([]byte(" "), n))
}

func hasPrefixAny(s string, p1, p2 string) bool {
	return (len(s) >= len(p1) && s[:len(p1)] == p1) ||
		(len(s) >= len(p2) && s[:len(p2)] == p2)
}

func stripOuterParens(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		depth := 0
		for i, r := range s {
			if r == '(' {
				depth++
			}
			if r == ')' {
				depth--
			}
			if depth == 0 && i != len(s)-1 {
				return s
			}
		}
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// ensureCStringLiteral guarantees a valid, quoted C string literal.
func ensureCStringLiteral(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return quoteCLike(s)
}

// quoteCLike adds quotes and escapes.
func quoteCLike(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				o1 := ((r >> 6) & 7) + '0'
				o2 := ((r >> 3) & 7) + '0'
				o3 := (r & 7) + '0'
				b.WriteByte('\\')
				b.WriteByte(byte(o1))
				b.WriteByte(byte(o2))
				b.WriteByte(byte(o3))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
