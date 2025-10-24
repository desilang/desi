package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
)

func openCodesJSON(t *testing.T) *os.File {
	t.Helper()
	// Try a few locations so this works no matter where `go test` is invoked from.
	candidates := []string{
		filepath.Join("..", "diag", "codes.json"),                   // from compiler/internal/check
		filepath.Join("compiler", "internal", "diag", "codes.json"), // from repo root
		filepath.Join(".", "codes.json"),                            // if run inside diag/ directly
	}
	for _, p := range candidates {
		if f, err := os.Open(p); err == nil {
			return f
		}
	}
	t.Fatalf("open codes.json: tried %v", candidates)
	return nil
}

func TestDiagCatalog_HasCheckerCodes(t *testing.T) {
	f := openCodesJSON(t)
	defer f.Close()

	cat, err := diag.LoadCatalog(f)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	expect := map[string]string{
		"type.no_matching_overload":  "DTE0101",
		"type.ambiguous_overload":    "DTE0102",
		"type.bad_pipeline_feed":     "DTE0103",
		"type.invalid_operand_types": "DTE0104",
		"type.not_callable":          "DTE0105",

		// sanity checks for existing ones we use
		"type.call_wrong_arity": "DTE0046",
		"type.undefined_name":   "DTE0001",
	}

	for path, wantID := range expect {
		e, ok := cat.Get(path)
		if !ok {
			t.Fatalf("missing entry in catalog: %s", path)
		}
		if e.ID != wantID {
			t.Fatalf("catalog ID mismatch for %s: got %s, want %s", path, e.ID, wantID)
		}
	}
}
