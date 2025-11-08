package format

import (
	"bytes"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

// FormatModule pretty-prints an AST.Module into source text.
// Phase M11/A: infrastructure only. A fuller node-by-node printer is added in Phase-1/2.
func FormatModule(m *ast.Module) ([]byte, error) {
	// For Task A we don't attempt AST-driven reflow yet.
	// The real implementation arrives with Phase-1/2 coverage tests.
	// Returning an empty buffer would be surprising; callers should use FormatBytes.
	return []byte{}, nil
}

// FormatBytes parses the input and returns formatted bytes on success.
// If the parser emits diagnostics, we return them and no bytes.
// In Phase A we conservatively normalize newlines, trailing space, and
// enforce tabs-at-BOL without changing token order.
func FormatBytes(src []byte) ([]byte, []diag.Diagnostic) {
	mod, diags := parse.ParseFile("<stdin>", src)
	if len(diags) > 0 {
		return nil, diags
	}
	_ = mod // reserved for Phase-1/2 AST-driven formatting

	out := normalizeSource(src)
	return out, nil
}

// ---- minimal normalizer (Task A) ----

// normalizeSource:
//   - trims trailing spaces on each line
//   - ensures exactly one newline at EOF
//   - converts leading groups of 4 spaces at BOL to tabs (tabs-only policy)
func normalizeSource(src []byte) []byte {
	lines := splitKeepLastNewline(src)
	var b bytes.Buffer
	for i, ln := range lines {
		// Drop trailing CR if present (normalize \r\n -> \n)
		if len(ln) >= 2 && ln[len(ln)-2] == '\r' && ln[len(ln)-1] == '\n' {
			ln = append(append([]byte{}, ln[:len(ln)-2]...), '\n')
		}
		// Trim trailing spaces (but keep final newline if present)
		hasNL := len(ln) > 0 && ln[len(ln)-1] == '\n'
		body := ln
		if hasNL {
			body = ln[:len(ln)-1]
		}
		body = trimRightSpaces(body)

		// tabs-only at BOL: transform leading 4-space groups into a tab.
		body = leadingSpacesToTabs(body)

		// Write line
		b.Write(body)
		if hasNL || i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	// Ensure exactly one trailing newline
	out := b.Bytes()
	out = bytes.TrimRight(out, "\n")
	out = append(out, '\n')
	return out
}

func splitKeepLastNewline(src []byte) [][]byte {
	s := string(src)
	if s == "" {
		return [][]byte{}
	}
	parts := strings.Split(s, "\n")
	res := make([][]byte, 0, len(parts))
	for i, p := range parts {
		if i == len(parts)-1 {
			// last chunk: only keep newline if source ended with newline
			if strings.HasSuffix(s, "\n") {
				res = append(res, []byte(p+"\n"))
			} else {
				res = append(res, []byte(p))
			}
		} else {
			res = append(res, []byte(p+"\n"))
		}
	}
	return res
}

func trimRightSpaces(b []byte) []byte {
	i := len(b)
	for i > 0 {
		if b[i-1] == ' ' || b[i-1] == '\t' {
			i--
			continue
		}
		break
	}
	return b[:i]
}

func leadingSpacesToTabs(b []byte) []byte {
	// Replace groups of 4 spaces at the start of the line with a single tab.
	i := 0
	var out bytes.Buffer
	// Count existing tabs first
	for i < len(b) && b[i] == '\t' {
		out.WriteByte('\t')
		i++
	}
	// Convert spaces in groups of 4
	for {
		if i+4 <= len(b) && b[i] == ' ' && b[i+1] == ' ' && b[i+2] == ' ' && b[i+3] == ' ' {
			out.WriteByte('\t')
			i += 4
			continue
		}
		break
	}
	// If any stray spaces remain before first non-space/tab, leave them as-is
	for i < len(b) && b[i] == ' ' {
		out.WriteByte(' ')
		i++
	}
	// Remainder
	out.Write(b[i:])
	return out.Bytes()
}

// ---- minimal writer scaffold for Phase-1/2 ----

type writer struct {
	buf    bytes.Buffer
	indent int
	bol    bool // beginning-of-line
}

func (w *writer) writeString(s string) {
	for i := 0; i < len(s); i++ {
		if w.bol {
			for j := 0; j < w.indent; j++ {
				w.buf.WriteByte('\t') // tabs-only at BOL
			}
			w.bol = false
		}
		ch := s[i]
		w.buf.WriteByte(ch)
		if ch == '\n' {
			w.bol = true
		}
	}
}

func (w *writer) newline() { w.writeString("\n") }
func (w *writer) indented(fn func()) {
	w.indent++
	defer func() { w.indent-- }()
	fn()
}
