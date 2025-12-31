package check

import "github.com/desilang/desi/compiler/internal/diag"

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
	}
	return diag.Diagnostic{
		CodeID:  codeID,
		Domain:  domain,
		Title:   "", // CLI may map the code to a title via catalog, else remains empty.
		Message: msg,
		Primary: diag.Label{Span: span, Primary: true},
	}
}
