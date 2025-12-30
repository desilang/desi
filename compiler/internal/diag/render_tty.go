package diag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/desilang/desi/compiler/internal/term"
)

// global render preferences controlled by CLIs
var globalFormat = "human"
var globalColor ColorMode = Auto

// SetGlobalRender allows CLIs to switch output behavior for all RenderTTY calls.
func SetGlobalRender(format string, color ColorMode) {
	switch format {
	case "json":
		globalFormat = "json"
	default:
		globalFormat = "human"
	}
	globalColor = color
}

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

// RenderTTY prints a diagnostic; legacy entry point with Theme.
// If JSON mode is requested globally, either capture it (for array flush) or stream a single object.
func (d Diagnostic) RenderTTY(w io.Writer, theme Theme) {
	if globalFormat == "json" {
		if jsonCaptureEnabled() {
			captureAdd(d)
			return
		}
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(toJSON(d))
		return
	}
	opt := Options{Color: globalColor, ExpandTabs: 8}
	if theme.Color {
		opt.Color = Always
	}
	_ = RenderTTYWith(w, d, osSource{}, opt)
}

// isTTYWriter reports whether w is a terminal (best-effort, no external deps).
func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	// Char device + sane TERM is a good proxy for a TTY without extra deps.
	if (st.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	_term := os.Getenv("TERM")
	return _term != "" && strings.ToLower(_term) != "dumb"
}

// RenderTTYWith renders a diagnostic to w using a SourceProvider and Options.
func RenderTTYWith(w io.Writer, d Diagnostic, src SourceProvider, opt Options) error {
	if opt.ExpandTabs <= 0 {
		opt.ExpandTabs = 8
	}

	// Determine color usage.
	useColor := false
	switch opt.Color {
	case Always:
		useColor = true
	case Never:
		useColor = false
	case Auto:
		// Honor NO_COLOR first.
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			useColor = false
		} else {
			// Enable when writing to an actual TTY.
			useColor = isTTYWriter(w)
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

	kind := "error"
	color := ansiBoldRed
	if d.Domain == "warn" {
		kind = "warning"
		color = ansiYellow
	}

	hdr := fmt.Sprintf("%s:%d:%d: %s[%s] %s: %s\n", file, line, col, kind, d.CodeID, d.Domain, title)
	if useColor {
		hdr = colorize(hdr, color)
	}
	_, _ = b.WriteString(hdr)

	// Primary label + snippet
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
		_, _ = fmt.Fprintf(&b, "  = note: unknown diagnostic code %q; add it to the catalog\n", d.CodeID)
	}

	term.Write(w, []byte(b.String()))
	return nil
}

func renderLabel(b *bytes.Buffer, l Label, src SourceProvider, opt Options, useColor bool, isPrimary bool) {
	if l.Span.File == "" || l.Span.Start.Line <= 0 {
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
	_, _ = fmt.Fprintf(b, "  %s\n", srcLine)

	caret := underlineForSpan(srcLine, l.Span.Start.Col, spanWidth(l), opt.ExpandTabs)
	if isPrimary && useColor {
		caret = colorize(caret, ansiBoldRed)
	} else if !isPrimary && useColor {
		caret = colorize(caret, ansiYellow)
	}
	if !isPrimary && l.Text != "" {
		caret += "  // note: " + l.Text
	}
	_, _ = fmt.Fprintf(b, "  %s\n", caret)
}

// underlineForSpan expands tabs visually (tabstop=opt.ExpandTabs) and draws ^~~~.
func underlineForSpan(srcLine string, startCol1 int, width int, tabstop int) string {
	if tabstop <= 0 {
		tabstop = 8
	}
	var sb strings.Builder
	col := 1
	for _, r := range srcLine {
		if col >= startCol1 {
			break
		}
		if r == '\t' {
			next := ((col-1)/tabstop+1)*tabstop + 1
			for col < next {
				sb.WriteByte(' ')
				col++
			}
		} else {
			sb.WriteByte(' ')
			col++
		}
	}
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
	if l.Span.Start.Line != l.Span.End.Line {
		return 1
	}
	start := _max(1, l.Span.Start.Col)
	end := _max(1, l.Span.End.Col)
	if end <= start {
		return 1
	}
	return end - start
}

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

// --- ANSI helpers ---
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
