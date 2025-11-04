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
func (d Diagnostic) RenderTTY(w io.Writer, theme Theme) {
	// Prefer catalog-provided fields when the diagnostic left them empty.
	FillFromCatalog(&d)

	sev := "error"
	if strings.HasPrefix(d.CodeID, "DW") {
		sev = "warning"
	}
	var b strings.Builder

	// Header: "error[CODE] title"
	if d.Title != "" {
		_, _ = fmt.Fprintf(&b, "%s[%s] %s\n", sev, d.CodeID, d.Title)
	} else if d.CodeID != "" {
		_, _ = fmt.Fprintf(&b, "%s[%s]\n", sev, d.CodeID)
	} else {
		// Fallback (shouldn't happen)
		_, _ = fmt.Fprintf(&b, "%s: diagnostic\n", sev)
	}

	// Primary location
	if (d.Primary != Label{}) && d.Primary.Span.File != "" {
		_, _ = fmt.Fprintf(&b, "  --> %s:%d:%d\n", d.Primary.Span.File, d.Primary.Span.Start.Line, d.Primary.Span.Start.Col)
		if d.Primary.Text != "" {
			_, _ = fmt.Fprintf(&b, "  = note: %s\n", d.Primary.Text)
		}
	}

	// Secondary labels (with their own locations)
	for _, l := range d.Labels {
		if l.Primary {
			continue
		}
		if l.Span.File != "" {
			_, _ = fmt.Fprintf(&b, "  = note: %s (%s:%d:%d)\n", l.Text, l.Span.File, l.Span.Start.Line, l.Span.Start.Col)
		} else {
			_, _ = fmt.Fprintf(&b, "  = note: %s\n", l.Text)
		}
	}

	// Notes
	for _, n := range d.Notes {
		_, _ = fmt.Fprintf(&b, "  = note: %s\n", n)
	}

	// Help (prefer catalog-backed help if Diagnostic.Help was empty)
	if d.Help != "" {
		_, _ = fmt.Fprintf(&b, "  = help: %s\n", d.Help)
	} else {
		// If help wasn't on the Diagnostic, try catalog (FillFromCatalog may
		// have already done this, but if not present on d, we still try to read).
		if e, ok := Lookup(d.CodeID); ok && e.Help != "" {
			_, _ = fmt.Fprintf(&b, "  = help: %s\n", e.Help)
		}
	}

	// Suggestions: only render if none were provided directly on the diagnostic.
	// (We don't currently store suggestions on Diagnostic; fetch from catalog.)
	if e, ok := Lookup(d.CodeID); ok && len(e.Suggestions) > 0 {
		for _, s := range e.Suggestions {
			where := "at location"
			switch s.Where.Kind {
			case "at":
				// If role is primary and we have a primary span, say so.
				if s.Where.Role == "primary" && (d.Primary != Label{}) {
					where = "at primary"
				} else {
					where = "at"
				}
			case "eol":
				where = "at end of line"
			case "bol":
				where = "at beginning of line"
			case "range":
				where = "at range"
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
		// Guardrail/fallback: unknown code in renderer (should not happen once tests are in place)
		_, _ = fmt.Fprintf(&b, "  = note: unknown diagnostic code %q; add it to the catalog\n", d.CodeID)
	}

	// Single buffered write to the chosen writer via term helper.
	term.Write(w, []byte(b.String()))
}
