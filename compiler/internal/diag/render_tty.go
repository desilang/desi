package diag

import (
	"fmt"
	"io"
)

type Theme struct {
	Color bool
}

// RenderTTY prints a human-friendly diagnostic; expand caret rendering later.
func (d Diagnostic) RenderTTY(w io.Writer, theme Theme) {
	code := d.CodeID
	title := d.Title
	msg := d.Message
	if msg == "" {
		msg = title
	}

	// Header
	fmt.Fprintf(w, "error[%s]: %s\n", code, msg)
	fmt.Fprintf(w, " --> %s:%d:%d\n", d.Primary.Span.File, d.Primary.Span.Start.Line, d.Primary.Span.Start.Col)
	fmt.Fprintf(w, "  |\n  | %s\n", d.Primary.Text)

	// Secondary labels
	for _, l := range d.Labels {
		if l.Primary {
			continue
		}
		fmt.Fprintf(w, "  = note: %s (%s:%d:%d)\n", l.Text, l.Span.File, l.Span.Start.Line, l.Span.Start.Col)
	}

	// Notes
	for _, n := range d.Notes {
		fmt.Fprintf(w, "  = note: %s\n", n)
	}

	// Help
	if d.Help != "" {
		fmt.Fprintf(w, "  = help: %s\n", d.Help)
	}
}
