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

// ConvertRawToNDJSON transforms "K|T|L|C" lines into NDJSON.
// If mirrorErr is true, it also emits LEXERR lines for ERR tokens.
func ConvertRawToNDJSON(raw string, mirrorErr bool) string {
	var b strings.Builder
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
		if mirrorErr && kind == "ERR" {
			msg := strings.ReplaceAll(text, `"`, `\"`)
			term.Eprintf("LEXERR line=%s col=%s msg=\"%s\"\n", line, col, msg)
		}
		// JSON-escape minimal
		esc := strings.ReplaceAll(text, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		term.Wprintf(&b, `{"kind":"%s","text":"%s","line":%s,"col":%s}`+"\n", kind, esc, line, col)
	}
	return b.String()
}
