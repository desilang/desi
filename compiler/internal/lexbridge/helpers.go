package lexbridge

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
)

// EscapeForDesiString converts arbitrary source into a safe Desi string literal.
func EscapeForDesiString(s string) string {
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
				// control char → emit as octal \ooo
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

// CopyFile reads from src and writes to dst (0600+rw-r--r--).
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// BuildTempTree ensures a clean temp import tree under gen/tmp/lexbridge,
// returning (tmpRoot, tmpWrapper, tmpLexerPath).
func BuildTempTree() (string, string, string, error) {
	tmpRoot := filepath.Join("gen", "tmp", "lexbridge")
	tmpWrapper := filepath.Join(tmpRoot, "main.desi")
	tmpImportDir := filepath.Join(tmpRoot, "compiler", "desi")
	tmpLexerPath := filepath.Join(tmpImportDir, "lexer.desi")

	_ = os.RemoveAll(tmpRoot)
	if err := os.MkdirAll(tmpImportDir, 0o755); err != nil {
		return "", "", "", err
	}
	return tmpRoot, tmpWrapper, tmpLexerPath, nil
}

// MirrorErrsToStderr scans raw "K|T|L|C" lines and writes LEXERR to stderr.
func MirrorErrsToStderr(raw string) {
	lines := strings.Split(raw, "\n")
	for _, ln := range lines {
		if ln == "" {
			continue
		}
		parts := strings.SplitN(ln, "|", 4)
		if len(parts) != 4 {
			continue
		}
		kind, text, line, col := parts[0], parts[1], parts[2], parts[3]
		if kind == "ERR" {
			// Escape quotes in message
			msg := strings.ReplaceAll(text, `"`, `\"`)
			term.Eprintf("LEXERR line=%s col=%s msg=\"%s\"\n", line, col, msg)
		}
	}
}

// ConvertRawToNDJSON turns the raw "KIND|TEXT|LINE|COL\n..." stream into NDJSON.
// Robust to embedded newlines in TEXT and Windows CRLF line endings.
// Also robust if the record terminator newline is missing: COL is parsed as digits only.
func ConvertRawToNDJSON(raw string, includeErrors bool) string {
	type state int
	const (
		sKind state = iota
		sText
		sLine
		sCol
	)

	var b strings.Builder
	i, n := 0, len(raw)

	// Helpers
	readUntil := func(delim byte) string {
		start := i
		for i < n && raw[i] != delim {
			i++
		}
		return raw[start:i]
	}
	isDigits := func(s string) bool {
		if s == "" {
			return false
		}
		for j := 0; j < len(s); j++ {
			if s[j] < '0' || s[j] > '9' {
				return false
			}
		}
		return true
	}
	esc := func(s string) string {
		var jb strings.Builder
		for _, r := range s {
			switch r {
			case '\\':
				jb.WriteString(`\\`)
			case '"':
				jb.WriteString(`\"`)
			case '\b':
				jb.WriteString(`\b`)
			case '\f':
				jb.WriteString(`\f`)
			case '\n':
				jb.WriteString(`\n`)
			case '\r':
				jb.WriteString(`\r`)
			case '\t':
				jb.WriteString(`\t`)
			default:
				if r < 0x20 {
					jb.WriteString(`\u00`)
					const hex = "0123456789ABCDEF"
					jb.WriteByte(hex[(r>>4)&0xF])
					jb.WriteByte(hex[r&0xF])
				} else {
					jb.WriteRune(r)
				}
			}
		}
		return jb.String()
	}

	for i < n {
		// Skip any stray record separators
		for i < n && (raw[i] == '\r' || raw[i] == '\n') {
			// consume CRLF or bare LF
			if raw[i] == '\r' {
				i++
				if i < n && raw[i] == '\n' {
					i++
				}
			} else {
				i++
			}
		}
		if i >= n {
			break
		}

		_ = sKind
		kind := readUntil('|')
		if i >= n {
			break
		}
		i++ // skip '|'

		_ = sText
		text := readUntil('|')
		if i >= n {
			break
		}
		i++ // skip '|'

		_ = sLine
		lineStr := readUntil('|')
		if i >= n {
			break
		}
		i++ // skip '|'

		// COL: scan digits ONLY (don't rely on newline)
		_ = sCol
		startCol := i
		for i < n && raw[i] >= '0' && raw[i] <= '9' {
			i++
		}
		colStr := raw[startCol:i]

		// Consume optional record terminator(s): CRLF or LF
		if i < n {
			if raw[i] == '\r' {
				i++
				if i < n && raw[i] == '\n' {
					i++
				}
			} else if raw[i] == '\n' {
				i++
			}
			// else: no newline — next byte is start of next token; that's fine.
		}

		if !includeErrors && kind == "ERR" {
			continue
		}

		// Emit NDJSON
		if isDigits(lineStr) && isDigits(colStr) {
			b.WriteString(`{"kind":"`)
			b.WriteString(kind)
			b.WriteString(`","text":"`)
			b.WriteString(esc(text))
			b.WriteString(`","line":`)
			b.WriteString(lineStr)
			b.WriteString(`,"col":`)
			b.WriteString(colStr)
			b.WriteString("}\n")
		} else {
			// fall back to strings if either field isn't purely digits
			b.WriteString(`{"kind":"`)
			b.WriteString(kind)
			b.WriteString(`","text":"`)
			b.WriteString(esc(text))
			b.WriteString(`","line":"`)
			b.WriteString(esc(lineStr))
			b.WriteString(`","col":"`)
			b.WriteString(esc(colStr))
			b.WriteString("\"}\n")
		}
	}

	return b.String()
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
