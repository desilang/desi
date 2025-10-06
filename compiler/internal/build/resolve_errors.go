package build

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// Build module-domain diagnostics with IDs/titles hydrated from codes.json.
// Keys assumed available: module.import_cycle (DME0001), module.not_found (DME0002)

// When we don't have a filename-aware renderer yet, include "at file:line:col"
// in the message or notes to keep users oriented.

func moduleImportCycleDiagAt(file string, line, col int, chain []string) diag.Diagnostic {
	ce, _ := diag.LookupFull("module", "import_cycle")
	code := ce.Entry.ID
	title := ce.Entry.Title
	if code == "" {
		code = "DME0001"
	}
	if title == "" {
		title = "import cycle detected"
	}
	short := filepath.Clean(file)
	msg := fmt.Sprintf("%s (at %s:%d:%d)", title, short, line, col)

	d := diag.Diagnostic{
		Domain:  "module",
		Key:     "import_cycle",
		Level:   diag.LevelError,
		Code:    code,
		Message: msg,
	}
	if len(chain) > 0 {
		d.Notes = append(d.Notes, "cycle: "+strings.Join(chain, " -> "))
	}
	return d
}

func moduleNotFoundDiagAt(module, file string, line, col int, lookedFor []string) diag.Diagnostic {
	ce, _ := diag.LookupFull("module", "not_found")
	code := ce.Entry.ID
	title := ce.Entry.Title
	if code == "" {
		code = "DME0002"
	}
	if title == "" {
		title = "cannot find module"
	}
	short := filepath.Clean(file)
	msg := fmt.Sprintf("%s: %q (import at %s:%d:%d)", title, module, short, line, col)

	d := diag.Diagnostic{
		Domain:  "module",
		Key:     "not_found",
		Level:   diag.LevelError,
		Code:    code,
		Message: msg,
	}
	if len(lookedFor) > 0 {
		d.Notes = append(d.Notes, "looked for:\n  "+strings.Join(lookedFor, "\n  "))
	}
	return d
}
