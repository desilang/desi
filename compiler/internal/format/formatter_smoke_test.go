package format

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestFormatter_Smoke_OnExamples(t *testing.T) {
	exdir := filepath.Join("..", "..", "..", "examples")
	files := []string{
		"14_m7_main.desi",
		"16_m8_async_lambda.desi",
		"23_range_map_filter.desi",
		"24_membership_len.desi",
		"25_comprehensions_lowered.desi",
	}
	for _, name := range files {
		p := filepath.Join(exdir, name)
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		out, diags := FormatBytes(src)
		if len(diags) != 0 {
			t.Fatalf("%s: parse diags: %+v", name, diags)
		}
		out2, diags2 := FormatBytes(out)
		if len(diags2) != 0 {
			t.Fatalf("%s: parse diags on formatted: %+v", name, diags2)
		}
		if !bytes.Equal(out, out2) {
			idx, line, col := firstDiffPos(out, out2)
			l1 := getLine(out, line)
			l2 := getLine(out2, line)
			ctx := contextLines(out, out2, line, 2)
			t.Fatalf("%s: not idempotent\nat byte=%d line=%d col=%d\npass1: %s\npass2: %s\n\ncontext:\n%s",
				name, idx, line, col, showVis(l1), showVis(l2), ctx)
		}
	}
}

// ----- helpers for debugging idempotence -----

func firstDiffPos(a, b []byte) (idx, line, col int) {
	line, col = 1, 1
	_max := len(a)
	if len(b) < _max {
		_max = len(b)
	}
	for i := 0; i < _max; i++ {
		if a[i] != b[i] {
			return i, line, col
		}
		if a[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	// one is a prefix of the other
	return _max, line, col
}

func getLine(src []byte, n int) []byte {
	if n < 1 {
		return nil
	}
	cur := 1
	start := 0
	for i, c := range src {
		if c == '\n' {
			if cur == n {
				return src[start:i]
			}
			cur++
			start = i + 1
		}
	}
	if cur == n {
		return src[start:]
	}
	return nil
}

func contextLines(a, b []byte, line, radius int) string {
	var sb bytes.Buffer
	start := line - radius
	if start < 1 {
		start = 1
	}
	end := line + radius
	for ln := start; ln <= end; ln++ {
		la := getLine(a, ln)
		lb := getLine(b, ln)
		// Stop when both are nil (past EOF in both)
		if la == nil && lb == nil {
			break
		}
		mark := "  "
		if ln == line {
			mark = ">>"
		}
		fmt.Fprintf(&sb, "%s L%03d A: %s\n", mark, ln, showVis(la))
		fmt.Fprintf(&sb, "%s L%03d B: %s\n", mark, ln, showVis(lb))
	}
	return sb.String()
}

func showVis(b []byte) string {
	if b == nil {
		return "<nil>"
	}
	// Make spaces/tabs/newlines visible; keep other UTF-8 intact.
	var out bytes.Buffer
	for len(b) > 0 {
		r, sz := utf8.DecodeRune(b)
		switch r {
		case '\t':
			out.WriteString("⇥")
		case ' ':
			out.WriteString("·")
		case '\n':
			out.WriteString("⏎")
		default:
			out.WriteRune(r)
		}
		b = b[sz:]
	}
	return out.String()
}
