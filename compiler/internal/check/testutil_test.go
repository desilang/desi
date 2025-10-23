package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
)

func mustNoDiags(t *testing.T, diags []diag.Diagnostic) {
	t.Helper()
	if len(diags) > 0 {
		var b strings.Builder
		for _, d := range diags {
			// keep it small; code id + short message
			b.WriteString(d.CodeID)
			if d.Message != "" {
				b.WriteString(": ")
				b.WriteString(d.Message)
			}
			b.WriteByte('\n')
		}
		t.Fatalf("expected no diagnostics, got %d:\n%s", len(diags), b.String())
	}
}

func mustHaveSomeDiagContaining(t *testing.T, diags []diag.Diagnostic, needle string) {
	t.Helper()
	for _, d := range diags {
		if strings.Contains(d.Message, needle) {
			return
		}
	}
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(d.CodeID)
		if d.Message != "" {
			b.WriteString(": ")
			b.WriteString(d.Message)
		}
		b.WriteByte('\n')
	}
	t.Fatalf("expected a diagnostic containing %q, got:\n%s", needle, b.String())
}
