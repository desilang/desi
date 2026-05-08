package check

import (
	"slices"

	"github.com/desilang/desi/compiler/internal/diag"
)

// Sync RAII warning codes (DSY0002-0005)
var syncWarningCodes = []string{"DSY0002", "DSY0003", "DSY0004", "DSY0005"}

// diagAt constructs a minimal Diagnostic with the given ID and primary span.
// We set the domain to "type" so it renders in that group when mapped by the CLI.
// Message is optional and may be empty.
func diagAt(codeID string, span diag.Span, msg string) diag.Diagnostic {
	domain := "type"
	if len(codeID) >= 2 {
		switch codeID[:2] {
		case "DW":
			domain = "warn"
		case "DTE":
			domain = "type"
		case "DCL":
			domain = "class"
		case "DME", "DMW":
			domain = "module"
		}
		// DSY0002-0005 are warnings for sync type RAII recommendations
		if slices.Contains(syncWarningCodes, codeID) {
			domain = "warn"
		}
		// DPR* are performance advisor warnings
		if len(codeID) >= 3 && codeID[:3] == "DPR" {
			domain = "perf"
		}
		// DPM* are build audit / permissions diagnostics
		if len(codeID) >= 3 && codeID[:3] == "DPM" {
			domain = "project"
		}
	}
	return diag.Diagnostic{
		CodeID:  codeID,
		Domain:  domain,
		Title:   "", // CLI may map the code to a title via catalog, else remains empty.
		Message: msg,
		Primary: diag.Label{Span: span, Primary: true},
	}
}
