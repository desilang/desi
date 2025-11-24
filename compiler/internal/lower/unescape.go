package lower

import "strings"

// unescapeString processes escape sequences in a string literal.
// Handles common escape sequences like \n, \t, \r, \\, \", etc.
func unescapeString(s string) string {
	// Use strings.Builder for efficient string concatenation
	var result strings.Builder
	result.Grow(len(s)) // Pre-allocate to avoid reallocations

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			// Process escape sequence
			switch s[i+1] {
			case 'n':
				result.WriteByte('\n')
				i++ // Skip the 'n'
			case 't':
				result.WriteByte('\t')
				i++
			case 'r':
				result.WriteByte('\r')
				i++
			case '\\':
				result.WriteByte('\\')
				i++
			case '"':
				result.WriteByte('"')
				i++
			case '0':
				result.WriteByte('\000')
				i++
			default:
				// Unknown escape sequence, keep the backslash
				result.WriteByte('\\')
			}
		} else {
			result.WriteByte(s[i])
		}
	}

	return result.String()
}
