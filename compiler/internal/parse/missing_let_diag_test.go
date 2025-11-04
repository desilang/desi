package parse

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
)

func TestMissingLet_Render_UsesCatalog(t *testing.T) {
	src := "def main() -> int:\n\tx = 1\n\t0\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) == 0 {
		t.Fatalf("expected at least one diagnostic, got none")
	}
	var found bool
	for _, d := range diags {
		if d.CodeID != "DPE0110" {
			continue
		}
		found = true
		var buf bytes.Buffer
		d.RenderTTY(&buf, diag.Theme{})
		out := buf.String()

		wantSubs := []string{
			"error[DPE0110]",
			"missing 'let' before variable declaration",
			"= help: Write 'let name = …' to declare a variable; plain '=' is not a statement operator.",
			"= suggestion (at primary): insert 'let '",
		}
		for _, sub := range wantSubs {
			if !strings.Contains(out, sub) {
				t.Fatalf("rendered output missing substring %q.\n--- got ---\n%s", sub, out)
			}
		}
		break
	}
	if !found {
		t.Fatalf("did not find DPE0110 among diagnostics: %+v", diags)
	}
}
