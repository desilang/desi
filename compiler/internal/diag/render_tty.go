package diag

import (
	"fmt"
	"io"
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
)

type Theme struct {
	Color bool
}

// RenderTTY prints a human-friendly diagnostic; expand caret rendering later.
// We format into a strings.Builder and write once via term.Write.
// All fmt.Fprintf calls explicitly discard return values to satisfy linters.
func (d Diagnostic) RenderTTY(w io.Writer, theme Theme) {
	var b strings.Builder

	code := d.CodeID
	title := d.Title
	msg := d.Message
	if msg == "" {
		msg = title
	}

	// Header
	_, _ = fmt.Fprintf(&b, "error[%s]: %s\n", code, msg)
	_, _ = fmt.Fprintf(&b, " --> %s:%d:%d\n", d.Primary.Span.File, d.Primary.Span.Start.Line, d.Primary.Span.Start.Col)
	_, _ = fmt.Fprintf(&b, "  |\n  | %s\n", d.Primary.Text)

	// Secondary labels
	for _, l := range d.Labels {
		if l.Primary {
			continue
		}
		_, _ = fmt.Fprintf(&b, "  = note: %s (%s:%d:%d)\n", l.Text, l.Span.File, l.Span.Start.Line, l.Span.Start.Col)
	}

	// Notes
	for _, n := range d.Notes {
		_, _ = fmt.Fprintf(&b, "  = note: %s\n", n)
	}

	// Help
	if d.Help != "" {
		_, _ = fmt.Fprintf(&b, "  = help: %s\n", d.Help)
	}

	// Single buffered write to the chosen writer via term helper.
	term.Write(w, []byte(b.String()))
}
