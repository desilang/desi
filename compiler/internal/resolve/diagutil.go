package resolve

import "github.com/desilang/desi/compiler/internal/diag"

// diagAt constructs a minimal Diagnostic for the "module" domain.
// Message is optional and may be empty; title is catalog-driven at render time.
func diagAt(codeID string, span diag.Span, msg string) diag.Diagnostic {
	return diag.Diagnostic{
		CodeID:  codeID,
		Domain:  "module",
		Title:   "",
		Message: msg,
		Primary: diag.Label{Span: span, Primary: true},
	}
}
