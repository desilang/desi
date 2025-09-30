package parsebridge

import (
	"strings"
)

func buildWrapper(entryAbs string) []byte {
	pathLit := escapeDesiString(entryAbs)
	var b strings.Builder
	b.WriteString("import compiler.desi.parser\n\n")
	b.WriteString("def main() -> int:\n")
	b.WriteString("  let path = ")
	b.WriteString(pathLit)
	b.WriteString("\n")
	b.WriteString("  let js = parse_to_json(path)\n")
	b.WriteString("  io.println(js)\n")
	b.WriteString("  return 0\n")
	b.WriteString("\n")
	return []byte(b.String())
}

func escapeDesiString(s string) string {
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
