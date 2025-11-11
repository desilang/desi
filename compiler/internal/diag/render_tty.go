package diag

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/desilang/desi/compiler/internal/term"
)

// Theme is kept for backward-compat with existing call sites.
// Setting Color=true forces color output (equivalent to Options{Color: Always}).
type Theme struct {
	Color bool
}

// ColorMode controls color emission.
type ColorMode int

const (
	Auto ColorMode = iota
	Always
	Never
)

// Options tunes TTY rendering.
type Options struct {
	Color      ColorMode // Auto|Always|Never
	Width      int       // soft wrap width; 0 disables wrapping
	ExpandTabs int       // visual tab stop (columns), default 8
}

// SourceProvider can return file bytes used to print source/underline.
// If File returns ok=false, the renderer will omit source snippets.
type SourceProvider interface {
	File(path string) (data []byte, ok bool)
}

// osSource implements SourceProvider by reading from disk.
type osSource struct{}

func (osSource) File(path string) ([]byte, bool) {
	if path == "" {
		return nil, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return b, true
}

// RenderTTY prints a human-friendly diagnostic; legacy entry point that accepts Theme.
// We translate to Options and use an on-disk source provider.
func (d Diagnostic) RenderTTY(w io.Writer, theme Theme) {
	opt := Options{Color: Auto, ExpandTabs: 8}
	if theme.Color {
		opt.Color = Always
	}
	_ = RenderTTYWith(w, d, osSource{}, opt)
}

// RenderTTYWith renders a diagnostic to w using a SourceProvider and Options.
func RenderTTYWith(w io.Writer, d Diagnostic, src SourceProvider, opt Options) error {
	if opt.ExpandTabs <= 0 {
		opt.ExpandTabs = 8
	}

	// Resolve color mode. In Auto, stay conservative (no color) unless explicitly forced.
	useColor := false
	switch opt.Color {
	case Always:
		useColor = true
	case Never:
		useColor = false
	case Auto:
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			useColor = false
		} else {
			useColor = false
		}
	}

	var b bytes.Buffer

	// Fill title/help from catalog if the Diagnostic didn't carry them.
	title := strings.TrimSpace(d.Title)
	if title == "" {
		if e, ok := Lookup(d.CodeID); ok && e.Title != "" {
			title = e.Title
		}
	}
	help := strings.TrimSpace(d.Help)
	if help == "" {
		if e, ok := Lookup(d.CodeID); ok && e.Help != "" {
			help = e.Help
		}
	}

	// Header: file:line:col: error[CODE] domain: title
	file := or(d.Primary.Span.File, "<stdin>")
	line := _max(1, d.Primary.Span.Start.Line)
	col := _max(1, d.Primary.Span.Start.Col)

	hdr := fmt.Sprintf("%s:%d:%d: error[%s] %s: %s\n", file, line, col, d.CodeID, d.Domain, title)
	if useColor {
		hdr = colorize(hdr, ansiBoldRed)
	}
	_, _ = b.WriteString(hdr)

	// Primary label + snippet (if available)
	renderLabel(&b, d.Primary, src, opt, useColor, true)

	// Secondary labels
	for _, l := range d.Labels {
		if l.Primary {
			continue
		}
		renderLabel(&b, l, src, opt, useColor, false)
	}

	// Notes
	for _, n := range d.Notes {
		if useColor {
			_, _ = fmt.Fprintf(&b, "  = %s: %s\n", colorize("note", ansiDim), n)
		} else {
			_, _ = fmt.Fprintf(&b, "  = note: %s\n", n)
		}
	}

	// Help (from diagnostic or catalog)
	if help != "" {
		if useColor {
			_, _ = fmt.Fprintf(&b, "  = %s: %s\n", colorize("help", ansiCyan), help)
		} else {
			_, _ = fmt.Fprintf(&b, "  = help: %s\n", help)
		}
	}

	// Suggestions (from catalog)
	if e, ok := Lookup(d.CodeID); ok && len(e.Suggestions) > 0 {
		for _, s := range e.Suggestions {
			where := "at location"
			switch s.Where.Kind {
			case "at":
				if s.Where.Role == "primary" && (d.Primary != Label{}) {
					where = "at primary"
				} else {
					where = "at"
				}
			case "eol":
				where = "at end of line"
			case "bol":
				where = "at start of line"
			}
			// Example:
			//   = suggestion (at primary): insert 'let ' — Add 'let ' before the name. [machine-applicable]
			_, _ = fmt.Fprintf(&b, "  = suggestion (%s): %s", where, s.Label)
			if s.Message != "" {
				_, _ = fmt.Fprintf(&b, " — %s", s.Message)
			}
			if s.Applicability != "" {
				_, _ = fmt.Fprintf(&b, " [%s]", s.Applicability)
			}
			_, _ = fmt.Fprint(&b, "\n")
		}
	} else if !Known(d.CodeID) && d.CodeID != "" {
		// Guardrail/fallback: unknown code in renderer
		_, _ = fmt.Fprintf(&b, "  = note: unknown diagnostic code %q; add it to the catalog\n", d.CodeID)
	}

	// Single buffered write via term helper (handles Windows consoles safely).
	term.Write(w, []byte(b.String()))
	return nil
}

func renderLabel(b *bytes.Buffer, l Label, src SourceProvider, opt Options, useColor bool, isPrimary bool) {
	if l.Span.File == "" || l.Span.Start.Line <= 0 {
		// No source context; just print a one-line note.
		tag := "note"
		if isPrimary {
			tag = "error"
		}
		if useColor {
			col := ansiBoldRed
			if !isPrimary {
				col = ansiYellow
			}
			_, _ = fmt.Fprintf(b, "  = %s: %s\n", colorize(tag, col), strings.TrimSpace(l.Text))
		} else {
			_, _ = fmt.Fprintf(b, "  = %s: %s\n", tag, strings.TrimSpace(l.Text))
		}
		return
	}

	data, ok := src.File(l.Span.File)
	if !ok {
		// Fallback: textual info without snippet.
		if l.Text != "" {
			_, _ = fmt.Fprintf(b, "  = note: %s (%s:%d:%d)\n",
				l.Text, or(l.Span.File, "<stdin>"), _max(1, l.Span.Start.Line), _max(1, l.Span.Start.Col))
		}
		return
	}

	lines := splitLines(data)
	idx := l.Span.Start.Line - 1
	if idx < 0 || idx >= len(lines) {
		return
	}
	srcLine := lines[idx]
	// Print the source line verbatim (preserve tabs)
	_, _ = fmt.Fprintf(b, "  %s\n", srcLine)

	// Build caret line with spaces only.
	caret := underlineForSpan(srcLine, l.Span.Start.Col, spanWidth(l), opt.ExpandTabs)
	if isPrimary && useColor {
		caret = colorize(caret, ansiBoldRed)
	} else if !isPrimary && useColor {
		caret = colorize(caret, ansiYellow)
	}
	// Optional end-of-line label for secondary
	if !isPrimary && l.Text != "" {
		caret += "  // note: " + l.Text
	}
	_, _ = fmt.Fprintf(b, "  %s\n", caret)
}

// underlineForSpan computes a caret/tilde underline for a given source line,
// 1-based start column, and width in columns. Tabs in the source are expanded
// to spaces for alignment but preserved in the printed source line.
func underlineForSpan(srcLine string, startCol1 int, width int, tabstop int) string {
	if tabstop <= 0 {
		tabstop = 8
	}
	// Convert visual columns up to the start into spaces.
	var sb strings.Builder
	col := 1
	for _, r := range srcLine {
		if col >= startCol1 {
			break
		}
		if r == '\t' {
			// Advance to next tab stop.
			next := ((col-1)/tabstop+1)*tabstop + 1
			for col < next {
				sb.WriteByte(' ')
				col++
			}
		} else {
			// Treat every rune as width 1 (combining-aware grapheme engine is out-of-scope for M12).
			sb.WriteByte(' ')
			col++
		}
	}
	// Write caret + tildes
	if width <= 1 {
		sb.WriteByte('^')
	} else {
		sb.WriteByte('^')
		for i := 1; i < width; i++ {
			sb.WriteByte('~')
		}
	}
	return sb.String()
}

func spanWidth(l Label) int {
	// fallback to 1 if positions are unset or degenerate
	if l.Span.Start.Line != l.Span.End.Line {
		// Multi-line span: we underline only the start column on the first line.
		return 1
	}
	start := _max(1, l.Span.Start.Col)
	end := _max(1, l.Span.End.Col)
	if end <= start {
		return 1
	}
	// width is (end - start) at minimum; we keep caret+~ for width>1
	return end - start
}

// splitLines returns lines without trailing newlines (normalizes CRLF).
func splitLines(b []byte) []string {
	var lines []string
	i := 0
	for i < len(b) {
		j := bytes.IndexByte(b[i:], '\n')
		if j < 0 {
			lines = append(lines, string(bytes.TrimRight(b[i:], "\r")))
			break
		}
		line := b[i : i+j]
		lines = append(lines, string(bytes.TrimRight(line, "\r")))
		i += j + 1
	}
	if len(b) == 0 {
		return []string{""}
	}
	return lines
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func _max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- ANSI helpers (minimal) ---

const (
	ansiReset   = "\x1b[0m"
	ansiBoldRed = "\x1b[1;31m"
	ansiYellow  = "\x1b[33m"
	ansiCyan    = "\x1b[36m"
	ansiDim     = "\x1b[2m"
)

func colorize(s string, code string) string {
	var b strings.Builder
	b.Grow(len(code) + len(s) + len(ansiReset))
	b.WriteString(code)
	b.WriteString(s)
	b.WriteString(ansiReset)
	return b.String()
}

// Keep these referenced.
var _ = utf8.UTFMax
var _ = strconv.IntSize
