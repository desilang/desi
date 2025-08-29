package lexbridge

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/lexer"
	"github.com/desilang/desi/compiler/internal/term"
)

// goToken is a simplified view of the Go lexer token for diffing.
type goToken struct {
	Kind string
	Text string
	Line int
	Col  int
}

// lexGoTokens lexes source with the existing Go lexer and returns a slice.
func lexGoTokens(src string) []goToken {
	lx := lexer.New(src)
	var out []goToken
	for {
		t := lx.Next()
		// t.Kind implements Stringer, so %s formatting yields a stable name.
		out = append(out, goToken{
			Kind: t.Kind.String(),
			Text: t.Lex,
			Line: t.Line,
			Col:  t.Col,
		})
		if t.Kind == lexer.TokEOF {
			break
		}
	}
	return out
}

// normalizeShort trims long texts and escapes newlines/tabs for one-line display.
func normalizeShort(s string) string {
	if len(s) > 40 {
		s = s[:37] + "..."
	}
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// DiffRow is one aligned line of go-vs-desi tokens (by index).
type DiffRow struct {
	Index int
	Go    goToken
	Desi  Token
}

// BuildLexDiff lexes with both lexers and aligns by index; the rows length is the max(len(go), len(desi)).
func BuildLexDiff(src string, desiNDJSON string) ([]DiffRow, error) {
	dtoks, err := ParseNDJSON(strings.NewReader(desiNDJSON))
	if err != nil {
		// Return what we have with a non-nil error so the caller can still print something.
		return nil, err
	}
	gtoks := lexGoTokens(src)

	n := len(gtoks)
	if len(dtoks) > n {
		n = len(dtoks)
	}
	rows := make([]DiffRow, n)
	for i := 0; i < n; i++ {
		var g goToken
		var d Token
		if i < len(gtoks) {
			g = gtoks[i]
		}
		if i < len(dtoks) {
			d = dtoks[i]
		}
		rows[i] = DiffRow{Index: i, Go: g, Desi: d}
	}
	return rows, nil
}

// FormatDiff pretty prints a side-by-side diff table.
// If limit>0, only the first limit rows are printed.
func FormatDiff(rows []DiffRow, limit int) string {
	var b strings.Builder

	// header
	term.Wprintf(&b, "%-6s | %-16s | %-30s || %-16s | %-30s\n", "idx", "Go KIND", "Go TEXT", "Desi KIND", "Desi TEXT")
	term.Wprintf(&b, "%s\n", strings.Repeat("-", 6+3+16+3+30+3+2+3+16+3+30))

	n := len(rows)
	if limit > 0 && limit < n {
		n = limit
	}
	for i := 0; i < n; i++ {
		r := rows[i]
		gText := normalizeShort(r.Go.Text)
		dText := normalizeShort(r.Desi.Text)

		// left labels include line:col for quick eyeballing
		goLbl := r.Go.Kind
		if goLbl == "" {
			goLbl = "—"
		}
		desiLbl := r.Desi.Kind
		if desiLbl == "" {
			desiLbl = "—"
		}

		term.Wprintf(&b, "%-6d | %-16s | %-30s || %-16s | %-30s\n",
			r.Index,
			goLbl,
			"'"+gText+"'",
			desiLbl,
			"'"+dText+"'",
		)
	}
	return b.String()
}
