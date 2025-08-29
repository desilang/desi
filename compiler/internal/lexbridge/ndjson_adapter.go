package lexbridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
)

// Token is a neutral token shape that mirrors the NDJSON schema
// produced by the Desi lexer bridge: {"kind":"...", "text":"...", "line":N, "col":M}
type Token struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// ParseNDJSON reads NDJSON tokens from r and returns a slice.
// Lines that fail to parse as JSON are ignored (but counted in the error).
func ParseNDJSON(r io.Reader) ([]Token, error) {
	var toks []Token
	sc := bufio.NewScanner(r)
	lineNo := 0
	var badLines []string

	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var t Token
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			// keep going; collect bad lines for diagnostics
			if len(badLines) < 5 {
				badLines = append(badLines, fmt.Sprintf("L%d: %s", lineNo, raw))
			}
			continue
		}
		toks = append(toks, t)
	}
	if err := sc.Err(); err != nil {
		return toks, err
	}
	if len(badLines) > 0 {
		return toks, fmt.Errorf("ignored %d malformed NDJSON line(s), first few: %s",
			len(badLines), strings.Join(badLines, " | "))
	}
	return toks, nil
}

// DebugFormat returns a readable dump similar to the Go lexer print style:
// "line:col  KIND  'text'"
func DebugFormat(toks []Token, limit int) string {
	var b strings.Builder
	n := len(toks)
	if limit > 0 && limit < n {
		n = limit
	}
	for i := 0; i < n; i++ {
		t := toks[i]
		txt := t.Text
		if len(txt) > 40 {
			txt = txt[:37] + "..."
		}
		if txt == "" {
			term.Wprintf(&b, "%d:%d  %s\n", t.Line, t.Col, t.Kind)
		} else {
			// single quotes for readability (not escaped)
			// replace newlines and tabs for one-line display
			clean := strings.ReplaceAll(txt, "\n", "\\n")
			clean = strings.ReplaceAll(clean, "\t", "\\t")
			term.Wprintf(&b, "%d:%d  %-8s  '%s'\n", t.Line, t.Col, t.Kind, clean)
		}
	}
	return b.String()
}
