package lexbridge

// quoteCLike returns a double-quoted literal with escapes suitable for our pipeline.
func quoteCLike(s string) string {
	var b []rune
	b = append(b, '"')
	for _, r := range s {
		switch r {
		case '\\':
			b = append(b, '\\', '\\')
		case '"':
			b = append(b, '\\', '"')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			if r < 0x20 {
				// octal escape \ooo
				o1 := ((r >> 6) & 7) + '0'
				o2 := ((r >> 3) & 7) + '0'
				o3 := (r & 7) + '0'
				b = append(b, '\\', rune(o1), rune(o2), rune(o3))
			} else {
				b = append(b, r)
			}
		}
	}
	b = append(b, '"')
	return string(b)
}
