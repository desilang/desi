package build

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// Build module-domain diagnostics with IDs/titles hydrated from codes.json.
// Keys assumed available: module.import_cycle (DME0001), module.not_found (DME0002)

func moduleImportCycleDiag() diag.Diagnostic {
	ce, _ := diag.LookupFull("module", "import_cycle")
	code := ce.Entry.ID
	title := ce.Entry.Title
	if code == "" {
		code = "DME0001"
	}
	if title == "" {
		title = "import cycle detected"
	}
	return diag.Diagnostic{
		Domain:  "module",
		Key:     "import_cycle",
		Level:   diag.LevelError,
		Code:    code,
		Message: title,
	}
}

func moduleNotFoundDiag(module string, lookedFor []string) diag.Diagnostic {
	ce, _ := diag.LookupFull("module", "not_found")
	code := ce.Entry.ID
	title := ce.Entry.Title
	if code == "" {
		code = "DME0002"
	}
	if title == "" {
		title = "cannot find module"
	}
	msg := fmt.Sprintf("%s: %q", title, module)
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
