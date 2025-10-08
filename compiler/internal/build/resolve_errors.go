package build

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// Build module-domain diagnostics with IDs/titles hydrated from codes.json.
// Keys assumed available: module.import_cycle (DME0001), module.missing_module (DME0002)

func moduleImportCycleDiagAt(file string, line, col int, chain []string) diag.Diagnostic {
	id, title := lookupIDTitle("module", "import_cycle", "DME0001", "import cycle")
	short := filepath.Clean(file)
	msg := fmt.Sprintf("%s (at %s:%d:%d)", title, short, line, col)

	d := diag.Diagnostic{
		Domain:  "module",
		Key:     "import_cycle",
		Level:   diag.LevelError,
		Code:    id,
		Message: msg,
	}
	if len(chain) > 0 {
		d.Notes = append(d.Notes, "cycle: "+strings.Join(chain, " -> "))
	}
	return d
}

func moduleNotFoundDiagAt(module, file string, line, col int, lookedFor []string) diag.Diagnostic {
	id, title := lookupIDTitle("module", "missing_module", "DME0002", "cannot find module")
	short := filepath.Clean(file)
	msg := fmt.Sprintf("%s: %q (import at %s:%d:%d)", title, module, short, line, col)

	d := diag.Diagnostic{
		Domain:  "module",
		Key:     "missing_module",
		Level:   diag.LevelError,
		Code:    id,
		Message: msg,
	}
	if len(lookedFor) > 0 {
		d.Notes = append(d.Notes, "looked for:\n  "+strings.Join(lookedFor, "\n  "))
	}
	return d
}
